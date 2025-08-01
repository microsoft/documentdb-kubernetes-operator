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
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

	log := log.FromContext(ctx)

	// Fetch the DocumentDB instance
	documentdb := &dbpreview.DocumentDB{}
	err := r.Get(ctx, req.NamespacedName, documentdb)

	if err != nil {
		if errors.IsNotFound(err) {
			// DocumentDB resource not found, handle cleanup
			log.Info("DocumentDB resource not found. Cleaning up associated resources.")
			if err := r.cleanupResources(ctx, req, documentdb); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get DocumentDB resource")
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
		enabled := !documentdb.Status.FailingOver
		ddbService := util.GetDocumentDBServiceDefinition(documentdb, req.Namespace, serviceType, enabled)

		// Check if the DocumentDB Service already exists for this instance
		foundService, err := util.UpsertService(ctx, r.Client, ddbService)
		if err != nil {
			log.Info("Failed to create DocumentDB Service; Requeuing.")
			return ctrl.Result{RequeueAfter: RequeueAfterShort}, nil
		}

		// Ensure DocumentDB Service has an IP assigned
		documentDbServiceIp, err = util.EnsureServiceIP(ctx, foundService)
		if err != nil {
			log.Info("DocumentDB Service IP not assigned, Requeuing.")
			return ctrl.Result{RequeueAfter: RequeueAfterShort}, nil
		}
	}

	// Ensure App ServiceAccount, Role and RoleBindings are created
	if err := r.EnsureServiceAccountRoleAndRoleBinding(ctx, documentdb, req.Namespace); err != nil {
		log.Info("Failed to create ServiceAccount, Role and RoleBinding; Requeuing.")
		return ctrl.Result{RequeueAfter: RequeueAfterShort}, nil
	}

	// create the CNPG Cluster
	documentdbImage := documentdb.Spec.DocumentDBImage
	if documentdbImage == "" {
		documentdbImage = util.DEFAULT_DOCUMENTDB_IMAGE
	}

	currentCnpgCluster := &cnpgv1.Cluster{}
	desiredCnpgCluster := cnpg.GetCnpgClusterSpec(req, *documentdb, documentdbImage, documentdb.Name, log)

	err = r.AddClusterReplicationToClusterSpec(ctx, *documentdb, desiredCnpgCluster)
	if err != nil {
		log.Error(err, "Failed to add physical replication features cnpg Cluster spec; Proceeding as single cluster.")
		return ctrl.Result{RequeueAfter: RequeueAfterShort}, nil
	}

	if err := r.Client.Get(ctx, types.NamespacedName{Name: desiredCnpgCluster.Name, Namespace: req.Namespace}, currentCnpgCluster); err != nil {
		if errors.IsNotFound(err) {
			if err := r.Client.Create(ctx, desiredCnpgCluster); err != nil {
				log.Error(err, "Failed to create CNPG Cluster")
				return ctrl.Result{RequeueAfter: RequeueAfterShort}, nil
			}
			log.Info("CNPG Cluster created successfully", "Cluster.Name", desiredCnpgCluster.Name, "Namespace", desiredCnpgCluster.Namespace)
			return ctrl.Result{RequeueAfter: RequeueAfterLong}, nil
		}
		log.Error(err, "Failed to get CNPG Cluster")
		return ctrl.Result{RequeueAfter: RequeueAfterShort}, nil
	}
	err, requeueTime := r.TryUpdateCluster(ctx, currentCnpgCluster, desiredCnpgCluster, documentdb)
	if err != nil {
		log.Error(err, "Failed to update CNPG Cluster")
	}
	if requeueTime > 0 {
		return ctrl.Result{RequeueAfter: requeueTime}, nil
	}

	// Update DocumentDB status with CNPG Cluster status and connection string
	if err := r.Client.Get(ctx, types.NamespacedName{Name: desiredCnpgCluster.Name, Namespace: req.Namespace}, currentCnpgCluster); err == nil {
		if currentCnpgCluster.Status.Phase != "" {
			documentdb.Status.Status = currentCnpgCluster.Status.Phase
			if documentDbServiceIp != "" {
				documentdb.Status.ConnectionString = util.GenerateConnectionString(documentdb, documentDbServiceIp)
			}
			if err := r.Status().Update(ctx, documentdb); err != nil {
				log.Error(err, "Failed to update DocumentDB status and connection string")
			}
		}
	}

	// TODO make this only happen on primary cluster, for now just edit the primary cluster's spec
	self, _, error := r.GetSelfAndSource(ctx, *documentdb)
	if error != nil {
		log.Error(error, "Failed to get self and source for DocumentDB")
		return ctrl.Result{RequeueAfter: RequeueAfterShort}, nil
	}
	isPrimary := documentdb.Spec.ClusterReplication.Primary == self

	// TODO make this only run once
	if currentCnpgCluster.Status.Phase == "Cluster in healthy state" && isPrimary {
		grantCommand := "GRANT documentdb_admin_role TO streaming_replica;"

		if err := r.executeSQLCommand(ctx, documentdb.Name, req.Namespace, grantCommand, "grant-permissions"); err != nil {
			log.Error(err, "Failed to grant permissions to streaming_replica")
			return ctrl.Result{RequeueAfter: RequeueAfterShort}, nil
		}
	}

	if isPrimary && documentdb.Status.FailingOver {
		log.Info("Still failing over")
		// Fenced above
		if currentCnpgCluster.Status.TargetPrimary == "azure-cluster-1" {

			// promote standby cluster to primary
			if err = Promote(ctx, r.Client, currentCnpgCluster.Namespace, currentCnpgCluster.Name, "azure-cluster-2"); err != nil {
				log.Error(err, "Failed to promote standby cluster to primary")
				return ctrl.Result{RequeueAfter: RequeueAfterShort}, nil
			}
		} else if currentCnpgCluster.Status.CurrentPrimary == "azure-cluster-2" {
			// create replication slot in replica
			log.Info("Creating wal_replication slot in new primary cluster")

			if err := r.executeSQLCommand(ctx, documentdb.Name, req.Namespace, "SELECT pg_create_physical_replication_slot('wal_replica');", "replication"); err != nil {
				log.Error(err, "Failed to create wal_replica replication slot")
				return ctrl.Result{RequeueAfter: RequeueAfterShort}, nil
			}

			log.Info("Marking failover as complete")
			documentdb.Status.FailingOver = false
			if err := r.Status().Update(ctx, documentdb); err != nil {
				log.Error(err, "Failed to update DocumentDB status")
				return ctrl.Result{RequeueAfter: RequeueAfterShort}, nil
			}
		}
	}

	return ctrl.Result{RequeueAfter: RequeueAfterLong}, nil
}

// cleanupResources handles the cleanup of associated resources when a DocumentDB resource is not found
func (r *DocumentDBReconciler) cleanupResources(ctx context.Context, req ctrl.Request, documentdb *dbpreview.DocumentDB) error {
	log := log.FromContext(ctx)

	// Cleanup DocumentDB Service
	if documentdb.Spec.ExposeViaService.ServiceType != "" {
		serviceName := util.DOCUMENTDB_SERVICE_PREFIX + req.Name
		if err := util.DeleteService(ctx, r.Client, serviceName, req.Namespace); err != nil {
			return err
		}
	}
	// Cleanup CNPG Cluster
	cnpgCluster := cnpg.GetCnpgClusterSpec(req, dbpreview.DocumentDB{}, "", req.Name, log)
	if err := r.Client.Delete(ctx, cnpgCluster); err != nil {
		if errors.IsNotFound(err) {
			log.Info("CNPG Cluster not found, skipping deletion.")
		} else {
			log.Error(err, "Failed to delete CNPG Cluster")
			return err
		}
	} else {
		log.Info("CNPG Cluster deleted successfully", "Cluster.Name", cnpgCluster.Name, "Namespace", cnpgCluster.Namespace)
	}

	// Cleanup ServiceAccount, Role and RoleBinding
	if err := util.DeleteRoleBinding(ctx, r.Client, req.Name, req.Namespace); err != nil {
		return err
	}
	if err := util.DeleteServiceAccount(ctx, r.Client, req.Name, req.Namespace); err != nil {
		return err
	}
	if err := util.DeleteRole(ctx, r.Client, req.Name, req.Namespace); err != nil {
		return err
	}

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
	var pod v1.Pod
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
func (r *DocumentDBReconciler) executeSQLCommand(ctx context.Context, documentdbName, namespace, sqlCommand, uniqueName string) error {
	zero := int32(0)
	sqlPod := &batchv1.Job{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-sql-executor", documentdbName, uniqueName),
			Namespace: namespace,
		},
		Spec: batchv1.JobSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					RestartPolicy: v1.RestartPolicyNever,
					Containers: []v1.Container{
						{
							Name:  "sql-executor",
							Image: "postgres:15",
							Command: []string{
								"psql",
								"-h", "azure-cluster-rw",
								"-U", "postgres",
								"-d", "postgres",
								"-c", sqlCommand,
							},
						},
					},
				},
			},
			TTLSecondsAfterFinished: &zero,
		},
	}

	if err := r.Client.Create(ctx, sqlPod); err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
	}

	return nil
}
