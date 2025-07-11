// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	"github.com/cloudnative-pg/cloudnative-pg/pkg/resources/status"
	pgTime "github.com/cloudnative-pg/machinery/pkg/postgres/time"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dbpreview "github.com/microsoft/documentdb-operator/api/preview"
	cnpg "github.com/microsoft/documentdb-operator/internal/cnpg"
	util "github.com/microsoft/documentdb-operator/internal/utils"
)

const (
	RequeueAfterShort = 10 * time.Second
	RequeueAfterLong  = 30 * time.Second
)

// DocumentDBReconciler reconciles a DocumentDB object
type DocumentDBReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

var reconcileMutex sync.Mutex

// +kubebuilder:rbac:groups=db.microsoft.com,resources=documentdbs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=db.microsoft.com,resources=documentdbs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=db.microsoft.com,resources=documentdbs/finalizers,verbs=update
func (r *DocumentDBReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reconcileMutex.Lock()
	defer reconcileMutex.Unlock()

	logger := log.FromContext(ctx)

	// Fetch the DocumentDB instance
	documentdb := &dbpreview.DocumentDB{}
	err := r.Get(ctx, req.NamespacedName, documentdb)
	if err != nil {
		if errors.IsNotFound(err) {
			// DocumentDB resource not found, handle cleanup
			logger.Info("DocumentDB resource not found. Cleaning up associated resources.")
			if err := r.cleanupResources(ctx, req, documentdb); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get DocumentDB resource")
		return ctrl.Result{}, err
	}

	replicationContext, err := util.GetReplicationContext(ctx, r.Client, *documentdb)
	if err != nil {
		logger.Error(err, "Failed to determine replication context")
		return ctrl.Result{}, err
	}

	var documentDbServiceIp string

	// Only create/manage the service if ExposeViaService is configured
	if documentdb.Spec.ExposeViaService.ServiceType != "" {
		serviceType := corev1.ServiceTypeClusterIP
		if documentdb.Spec.ExposeViaService.ServiceType == "LoadBalancer" {
			serviceType = corev1.ServiceTypeLoadBalancer // Public LoadBalancer service
		}

		// Define the Service for this DocumentDB instance
		ddbService := util.GetDocumentDBServiceDefinition(documentdb, replicationContext, req.Namespace, serviceType)

		// Check if the DocumentDB Service already exists for this instance
		foundService, err := util.UpsertService(ctx, r.Client, ddbService)
		if err != nil {
			logger.Error(err, "Failed to create DocumentDB Service; Requeuing.")
			return ctrl.Result{RequeueAfter: RequeueAfterShort}, nil
		}

		// Ensure DocumentDB Service has an IP assigned
		documentDbServiceIp, err = util.EnsureServiceIP(ctx, foundService)
		if err != nil {
			logger.Info("DocumentDB Service IP not assigned, Requeuing.")
			return ctrl.Result{RequeueAfter: RequeueAfterShort}, nil
		}
	}

	// Ensure App ServiceAccount, Role and RoleBindings are created
	if err := r.EnsureServiceAccountRoleAndRoleBinding(ctx, documentdb, req.Namespace); err != nil {
		logger.Info("Failed to create ServiceAccount, Role and RoleBinding; Requeuing.")
		return ctrl.Result{RequeueAfter: RequeueAfterShort}, nil
	}

	// create the CNPG Cluster
	documentdbImage := util.GetDocumentDBImageForInstance(documentdb)

	currentCnpgCluster := &cnpgv1.Cluster{}
	desiredCnpgCluster := cnpg.GetCnpgClusterSpec(req, documentdb, documentdbImage, documentdb.Name, replicationContext.StorageClass, logger)

	if replicationContext.IsReplicating() {
		err = r.AddClusterReplicationToClusterSpec(ctx, documentdb, replicationContext, desiredCnpgCluster)
		if err != nil {
			logger.Error(err, "Failed to add physical replication features cnpg Cluster spec; Proceeding as single cluster.")
			return ctrl.Result{RequeueAfter: RequeueAfterShort}, nil
		}
	}

	if err := r.Client.Get(ctx, types.NamespacedName{Name: desiredCnpgCluster.Name, Namespace: req.Namespace}, currentCnpgCluster); err != nil {
		if errors.IsNotFound(err) {
			if err := r.Client.Create(ctx, desiredCnpgCluster); err != nil {
				logger.Error(err, "Failed to create CNPG Cluster")
				return ctrl.Result{RequeueAfter: RequeueAfterShort}, nil
			}
			logger.Info("CNPG Cluster created successfully", "Cluster.Name", desiredCnpgCluster.Name, "Namespace", desiredCnpgCluster.Namespace)
			return ctrl.Result{RequeueAfter: RequeueAfterLong}, nil
		}
		logger.Error(err, "Failed to get CNPG Cluster")
		return ctrl.Result{RequeueAfter: RequeueAfterShort}, nil
	}

	// Check if anything has changed in the generated cnpg spec
	err, requeueTime := r.TryUpdateCluster(ctx, currentCnpgCluster, desiredCnpgCluster, documentdb, replicationContext)
	if err != nil {
		logger.Error(err, "Failed to update CNPG Cluster")
	}
	if requeueTime > 0 {
		return ctrl.Result{RequeueAfter: requeueTime}, nil
	}

	// Update DocumentDB status with CNPG Cluster status and connection string
	if err := r.Client.Get(ctx, types.NamespacedName{Name: desiredCnpgCluster.Name, Namespace: req.Namespace}, currentCnpgCluster); err == nil {
		// Ensure plugin enabled and TLS secret parameter kept in sync once ready
		if documentdb.Status.TLS != nil && documentdb.Status.TLS.Ready && documentdb.Status.TLS.SecretName != "" {
			logger.Info("Syncing TLS secret into CNPG Cluster plugin parameters", "secret", documentdb.Status.TLS.SecretName)
			updated := false
			for i := range currentCnpgCluster.Spec.Plugins {
				p := &currentCnpgCluster.Spec.Plugins[i]
				if p.Name == desiredCnpgCluster.Spec.Plugins[0].Name { // target our sidecar plugin
					if p.Enabled == nil || !*p.Enabled {
						trueVal := true
						p.Enabled = &trueVal
						updated = true
						logger.Info("Enabled sidecar plugin")
					}
					if p.Parameters == nil {
						p.Parameters = map[string]string{}
					}
					currentVal := p.Parameters["gatewayTLSSecret"]
					if currentVal != documentdb.Status.TLS.SecretName {
						p.Parameters["gatewayTLSSecret"] = documentdb.Status.TLS.SecretName
						updated = true
						logger.Info("Updated gatewayTLSSecret parameter", "old", currentVal, "new", documentdb.Status.TLS.SecretName)
					}
				}
			}
			if updated {
				if currentCnpgCluster.Annotations == nil {
					currentCnpgCluster.Annotations = map[string]string{}
				}
				currentCnpgCluster.Annotations["db.microsoft.com/gateway-tls-rev"] = time.Now().Format(time.RFC3339Nano)
				if err := r.Client.Update(ctx, currentCnpgCluster); err == nil {
					logger.Info("Patched CNPG Cluster with TLS settings; requeueing for pod update")
					return ctrl.Result{RequeueAfter: RequeueAfterShort}, nil
				} else {
					logger.Error(err, "Failed to update CNPG Cluster with TLS settings")
				}
			}

			if err := r.Status().Update(ctx, documentdb); err != nil {
				logger.Error(err, "Failed to update DocumentDB status and connection string")
			}
		}
	}

	if currentCnpgCluster.Status.Phase == "Cluster in healthy state" && replicationContext.IsPrimary() {
		grantCommand := "GRANT documentdb_admin_role TO streaming_replica;"

		if err := r.executeSQLCommand(ctx, documentdb, replicationContext, grantCommand, "grant-permissions"); err != nil {
			logger.Error(err, "Failed to grant permissions to streaming_replica")
			return ctrl.Result{RequeueAfter: RequeueAfterShort}, nil
		}
	}

	if replicationContext.IsPrimary() && documentdb.Status.TargetPrimary != "" {
		// If these are different, we need to initiate a failover
		if documentdb.Status.TargetPrimary != currentCnpgCluster.Status.TargetPrimary {

			if err = Promote(ctx, r.Client, currentCnpgCluster.Namespace, currentCnpgCluster.Name, documentdb.Status.TargetPrimary); err != nil {
				logger.Error(err, "Failed to promote standby cluster to primary")
				return ctrl.Result{RequeueAfter: RequeueAfterShort}, nil
			}
		} else if documentdb.Status.TargetPrimary != documentdb.Status.LocalPrimary &&
			documentdb.Status.TargetPrimary == currentCnpgCluster.Status.CurrentPrimary {

			logger.Info("Marking failover as complete")
			documentdb.Status.LocalPrimary = currentCnpgCluster.Status.CurrentPrimary
			if err := r.Status().Update(ctx, documentdb); err != nil {
				logger.Error(err, "Failed to update DocumentDB status")
				return ctrl.Result{RequeueAfter: RequeueAfterShort}, nil
			}
		}
		// Update status connection string
		if documentDbServiceIp != "" {
			trustTLS := documentdb.Status.TLS != nil && documentdb.Status.TLS.Ready
			documentdb.Status.ConnectionString = util.GenerateConnectionString(documentdb, documentDbServiceIp, trustTLS)
		}
		if err := r.Status().Update(ctx, documentdb); err != nil {
			logger.Error(err, "Failed to update DocumentDB status and connection string")
		}
	}

	return ctrl.Result{RequeueAfter: RequeueAfterLong}, nil
}

// cleanupResources handles the cleanup of associated resources when a DocumentDB resource is not found
func (r *DocumentDBReconciler) cleanupResources(ctx context.Context, req ctrl.Request, documentdb *dbpreview.DocumentDB) error {
	log := log.FromContext(ctx)

	// Cleanup ServiceAccount, Role and RoleBinding
	if err := util.DeleteRoleBinding(ctx, r.Client, req.Name, req.Namespace); err != nil {
		log.Error(err, "Failed to delete RoleBinding during cleanup", "RoleBindingName", req.Name)
		// Continue with other cleanup even if this fails
	}

	if err := util.DeleteServiceAccount(ctx, r.Client, req.Name, req.Namespace); err != nil {
		log.Error(err, "Failed to delete ServiceAccount during cleanup", "ServiceAccountName", req.Name)
		// Continue with other cleanup even if this fails
	}

	if err := util.DeleteRole(ctx, r.Client, req.Name, req.Namespace); err != nil {
		log.Error(err, "Failed to delete Role during cleanup", "RoleName", req.Name)
		// Continue with other cleanup even if this fails
	}

	log.Info("Cleanup process completed", "DocumentDB", req.Name, "Namespace", req.Namespace)
	return nil
}

func (r *DocumentDBReconciler) EnsureServiceAccountRoleAndRoleBinding(ctx context.Context, documentdb *dbpreview.DocumentDB, namespace string) error {
	log := log.FromContext(ctx)

	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"pods", "services", "endpoints"},
			Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
		},
	}

	// Create Role
	if err := util.CreateRole(ctx, r.Client, documentdb.Name, namespace, rules); err != nil {
		log.Error(err, "Failed to create Role for DocumentDB", "DocumentDB.Name", documentdb.Name, "Namespace", namespace)
		return err
	}

	// Create ServiceAccount
	if err := util.CreateServiceAccount(ctx, r.Client, documentdb.Name, namespace); err != nil {
		log.Error(err, "Failed to create ServiceAccount for DocumentDB", "DocumentDB.Name", documentdb.Name, "Namespace", namespace)
		return err
	}

	// Create RoleBinding
	if err := util.CreateRoleBinding(ctx, r.Client, documentdb.Name, namespace); err != nil {
		log.Error(err, "Failed to create RoleBinding for DocumentDB", "DocumentDB.Name", documentdb.Name, "Namespace", namespace)
		return err
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DocumentDBReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dbpreview.DocumentDB{}).
		Owns(&corev1.Service{}).
		Owns(&cnpgv1.Cluster{}).
		Owns(&cnpgv1.Publication{}).
		Owns(&cnpgv1.Subscription{}).
		Named("documentdb-controller").
		Complete(r)
}

// COPIED FROM https://github.com/cloudnative-pg/cloudnative-pg/blob/release-1.25/internal/cmd/plugin/promote/promote.go
func Promote(ctx context.Context, cli client.Client,
	namespace, clusterName, serverName string,
) error {
	var cluster cnpgv1.Cluster

	log := log.FromContext(ctx)

	// Get the Cluster object
	err := cli.Get(ctx, client.ObjectKey{Namespace: namespace, Name: clusterName}, &cluster)
	if err != nil {
		return fmt.Errorf("cluster %s not found in namespace %s: %w", clusterName, namespace, err)
	}

	log.Info("Promoting new primary node", "serverName", serverName, "clusterName", clusterName)

	// If server name is equal to target primary, there is no need to promote
	// that instance
	if cluster.Status.TargetPrimary == serverName {
		fmt.Printf("%s is already the primary node in the cluster\n", serverName)
		return nil
	}

	// Check if the Pod exist
	var pod corev1.Pod
	err = cli.Get(ctx, client.ObjectKey{Namespace: namespace, Name: serverName}, &pod)
	if err != nil {
		return fmt.Errorf("new primary node %s not found in namespace %s: %w", serverName, namespace, err)
	}

	// The Pod exists, let's update the cluster's status with the new target primary
	reconcileTargetPrimaryFunc := func(cluster *cnpgv1.Cluster) {
		cluster.Status.TargetPrimary = serverName
		cluster.Status.TargetPrimaryTimestamp = pgTime.GetCurrentTimestamp()
		cluster.Status.Phase = cnpgv1.PhaseSwitchover
		cluster.Status.PhaseReason = fmt.Sprintf("Switching over to %v", serverName)
	}
	if err := status.PatchWithOptimisticLock(ctx, cli, &cluster,
		reconcileTargetPrimaryFunc,
		status.SetClusterReadyConditionTX,
	); err != nil {
		return err
	}
	log.Info("Promotion in progress for ", "New primary", serverName, "cluster name", clusterName)
	return nil
}

// executeSQLCommand creates a pod to execute SQL commands against the azure-cluster-rw service
// TODO: Should find a less intrusive way to do this with CNPG
func (r *DocumentDBReconciler) executeSQLCommand(ctx context.Context, documentdb *dbpreview.DocumentDB, replicationContext *util.ReplicationContext, sqlCommand, uniqueName string) error {
	zero := int32(0)
	host := replicationContext.Self + "-rw"
	sqlPod := &batchv1.Job{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-sql-executor", documentdb.Name, uniqueName),
			Namespace: documentdb.Namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:  "sql-executor",
							Image: documentdb.Spec.DocumentDBImage,
							Command: []string{
								"psql",
								"-h", host,
								"-U", "postgres",
								"-d", "postgres",
								"-c", sqlCommand,
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu":    resource.MustParse(util.SQL_JOB_REQUESTS_CPU),
									"memory": resource.MustParse(util.SQL_JOB_REQUESTS_MEMORY),
								},
								Limits: corev1.ResourceList{
									"cpu":    resource.MustParse(util.SQL_JOB_LIMITS_CPU),
									"memory": resource.MustParse(util.SQL_JOB_LIMITS_MEMORY),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:                ptr.To(int64(util.SQL_JOB_LINUX_UID)),
								RunAsNonRoot:             ptr.To(util.SQL_JOB_RUN_AS_NON_ROOT),
								AllowPrivilegeEscalation: ptr.To(util.SQL_JOB_ALLOW_PRIVILEGED),
							},
						},
					},
				},
			},
			TTLSecondsAfterFinished: &zero,
		},
	}

	if replicationContext.IsIstioNetworking() {
		sqlPod.Spec.Template.ObjectMeta =
			ctrl.ObjectMeta{
				Annotations: map[string]string{
					"sidecar.istio.io/inject": "false",
				},
			}
	}

	if err := r.Client.Create(ctx, sqlPod); err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
	}

	return nil
}
