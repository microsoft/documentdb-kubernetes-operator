// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package controller

import (
	"context"
	"sync"
	"time"

	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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

// reconcileGatewayTLS handles self-signed TLS provisioning (SelfSigned mode) for the gateway.
// Future: extend for CertManager and Provided modes.
func (r *DocumentDBReconciler) reconcileGatewayTLS(ctx context.Context, ddb *dbpreview.DocumentDB) error {
	if ddb.Spec.TLS == nil || ddb.Spec.TLS.Mode == "" || ddb.Spec.TLS.Mode == "Disabled" {
		return nil
	}
	// Initialize status structure if missing
	if ddb.Status.TLS == nil {
		ddb.Status.TLS = &dbpreview.TLSStatus{Ready: false}
		_ = r.Status().Update(ctx, ddb)
	}

	switch ddb.Spec.TLS.Mode {
	case "SelfSigned":
		return r.ensureSelfSignedCert(ctx, ddb)
	case "Provided":
		return r.ensureProvidedSecret(ctx, ddb)
	case "CertManager":
		return r.ensureCertManagerManagedCert(ctx, ddb)
	default:
		return nil
	}
}

// ensureProvidedSecret validates presence of a user-provided secret and marks TLS ready.
func (r *DocumentDBReconciler) ensureProvidedSecret(ctx context.Context, ddb *dbpreview.DocumentDB) error {
	if ddb.Status.TLS == nil { // defensive init for direct test invocation
		ddb.Status.TLS = &dbpreview.TLSStatus{}
	}
	if ddb.Spec.TLS == nil || ddb.Spec.TLS.Provided == nil || ddb.Spec.TLS.Provided.SecretName == "" {
		ddb.Status.TLS.Message = "Provided TLS secret name missing"
		_ = r.Status().Update(ctx, ddb)
		return nil
	}
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: ddb.Spec.TLS.Provided.SecretName, Namespace: ddb.Namespace}, secret); err != nil {
		if errors.IsNotFound(err) {
			ddb.Status.TLS.Ready = false
			ddb.Status.TLS.SecretName = ddb.Spec.TLS.Provided.SecretName
			ddb.Status.TLS.Message = "Waiting for provided TLS secret"
			_ = r.Status().Update(ctx, ddb)
			return nil
		}
		return err
	}
	// Basic key presence check
	if _, crtOk := secret.Data["tls.crt"]; !crtOk {
		ddb.Status.TLS.Ready = false
		ddb.Status.TLS.Message = "Provided secret missing tls.crt"
		_ = r.Status().Update(ctx, ddb)
		return nil
	}
	if _, keyOk := secret.Data["tls.key"]; !keyOk {
		ddb.Status.TLS.Ready = false
		ddb.Status.TLS.Message = "Provided secret missing tls.key"
		_ = r.Status().Update(ctx, ddb)
		return nil
	}
	ddb.Status.TLS.Ready = true
	ddb.Status.TLS.SecretName = ddb.Spec.TLS.Provided.SecretName
	ddb.Status.TLS.Message = "Using provided TLS secret"
	_ = r.Status().Update(ctx, ddb)
	return nil
}

// ensureCertManagerManagedCert ensures a certificate using a user-specified Issuer/ClusterIssuer.
func (r *DocumentDBReconciler) ensureCertManagerManagedCert(ctx context.Context, ddb *dbpreview.DocumentDB) error {
	if ddb.Status.TLS == nil { // defensive init for direct test invocation
		ddb.Status.TLS = &dbpreview.TLSStatus{}
	}
	if ddb.Spec.TLS == nil || ddb.Spec.TLS.CertManager == nil {
		ddb.Status.TLS.Message = "CertManager configuration missing"
		_ = r.Status().Update(ctx, ddb)
		return nil
	}
	cmCfg := ddb.Spec.TLS.CertManager

	issuerRef := cmmeta.ObjectReference{Name: cmCfg.IssuerRef.Name}
	if cmCfg.IssuerRef.Kind != "" {
		issuerRef.Kind = cmCfg.IssuerRef.Kind
	} else {
		issuerRef.Kind = "Issuer"
	}
	if cmCfg.IssuerRef.Group != "" {
		issuerRef.Group = cmCfg.IssuerRef.Group
	} else {
		issuerRef.Group = "cert-manager.io"
	}

	// Determine secret name
	secretName := cmCfg.SecretName
	if secretName == "" {
		secretName = ddb.Name + "-gateway-cert-tls"
	}

	// Build DNS names: include requested + service names
	serviceBase := util.DOCUMENTDB_SERVICE_PREFIX + ddb.Name
	baseDNS := []string{serviceBase, serviceBase + "." + ddb.Namespace, serviceBase + "." + ddb.Namespace + ".svc"}
	dnsSet := map[string]struct{}{}
	finalDNS := []string{}
	for _, n := range cmCfg.DNSNames {
		if _, ok := dnsSet[n]; !ok && n != "" {
			dnsSet[n] = struct{}{}
			finalDNS = append(finalDNS, n)
		}
	}
	for _, n := range baseDNS {
		if _, ok := dnsSet[n]; !ok {
			dnsSet[n] = struct{}{}
			finalDNS = append(finalDNS, n)
		}
	}

	certName := ddb.Name + "-gateway-cert" // stable logical cert name
	cert := &cmapi.Certificate{}
	if err := r.Get(ctx, types.NamespacedName{Name: certName, Namespace: ddb.Namespace}, cert); err != nil {
		cert = &cmapi.Certificate{
			ObjectMeta: metav1.ObjectMeta{Name: certName, Namespace: ddb.Namespace},
			Spec: cmapi.CertificateSpec{
				SecretName:  secretName,
				DNSNames:    finalDNS,
				IssuerRef:   issuerRef,
				Duration:    &metav1.Duration{Duration: 90 * 24 * time.Hour},
				RenewBefore: &metav1.Duration{Duration: 15 * 24 * time.Hour},
				Usages:      []cmapi.KeyUsage{cmapi.UsageServerAuth},
			},
		}
		// Ensure the certificate is owned by the DocumentDB resource for GC cleanup
		_ = controllerutil.SetControllerReference(ddb, cert, r.Scheme)
		if createErr := r.Create(ctx, cert); createErr != nil {
			return createErr
		}
		ddb.Status.TLS.SecretName = secretName
		ddb.Status.TLS.Message = "Creating cert-manager certificate"
		_ = r.Status().Update(ctx, ddb)
		return nil
	}
	// readiness check
	for _, cond := range cert.Status.Conditions {
		if cond.Type == cmapi.CertificateConditionReady && cond.Status == cmmeta.ConditionTrue {
			if !ddb.Status.TLS.Ready {
				ddb.Status.TLS.Ready = true
				ddb.Status.TLS.SecretName = cert.Spec.SecretName
				ddb.Status.TLS.Message = "Gateway TLS certificate ready (cert-manager)"
				_ = r.Status().Update(ctx, ddb)
			}
			return nil
		}
	}
	ddb.Status.TLS.SecretName = cert.Spec.SecretName
	ddb.Status.TLS.Message = "Waiting for cert-manager certificate to become ready"
	_ = r.Status().Update(ctx, ddb)
	return nil
}

func (r *DocumentDBReconciler) ensureSelfSignedCert(ctx context.Context, ddb *dbpreview.DocumentDB) error {
	if ddb.Status.TLS == nil { // defensive init for direct test invocation
		ddb.Status.TLS = &dbpreview.TLSStatus{}
	}
	namespace := ddb.Namespace
	issuerName := ddb.Name + "-gateway-selfsigned"
	certName := ddb.Name + "-gateway-cert"
	// Determine secret name (reuse cert name)
	secretName := certName + "-tls"

	// Create Issuer if absent
	issuer := &cmapi.Issuer{}
	if err := r.Get(ctx, types.NamespacedName{Name: issuerName, Namespace: namespace}, issuer); err != nil {
		issuer = &cmapi.Issuer{
			ObjectMeta: metav1.ObjectMeta{Name: issuerName, Namespace: namespace},
			Spec:       cmapi.IssuerSpec{IssuerConfig: cmapi.IssuerConfig{SelfSigned: &cmapi.SelfSignedIssuer{}}},
		}
		// Ensure the issuer is owned by the DocumentDB resource for GC cleanup
		_ = controllerutil.SetControllerReference(ddb, issuer, r.Scheme)
		_ = r.Create(ctx, issuer)
	}

	// Build DNS names for service
	serviceBase := util.DOCUMENTDB_SERVICE_PREFIX + ddb.Name
	dnsNames := []string{
		serviceBase,
		serviceBase + "." + namespace,
		serviceBase + "." + namespace + ".svc",
	}

	cert := &cmapi.Certificate{}
	if err := r.Get(ctx, types.NamespacedName{Name: certName, Namespace: namespace}, cert); err != nil {
		cert = &cmapi.Certificate{
			ObjectMeta: metav1.ObjectMeta{Name: certName, Namespace: namespace},
			Spec: cmapi.CertificateSpec{
				SecretName:  secretName,
				Duration:    &metav1.Duration{Duration: 90 * 24 * time.Hour},
				RenewBefore: &metav1.Duration{Duration: 15 * 24 * time.Hour},
				DNSNames:    dnsNames,
				IssuerRef:   cmmeta.ObjectReference{Name: issuerName, Kind: "Issuer", Group: "cert-manager.io"},
				Usages:      []cmapi.KeyUsage{cmapi.UsageServerAuth},
			},
		}
		// Ensure the certificate is owned by the DocumentDB resource for GC cleanup
		_ = controllerutil.SetControllerReference(ddb, cert, r.Scheme)
		if createErr := r.Create(ctx, cert); createErr != nil {
			return createErr
		}
		ddb.Status.TLS.SecretName = secretName
		ddb.Status.TLS.Message = "Creating self-signed certificate"
		_ = r.Status().Update(ctx, ddb)
		return nil
	}

	// Evaluate readiness
	for _, cond := range cert.Status.Conditions {
		if cond.Type == cmapi.CertificateConditionReady && cond.Status == cmmeta.ConditionTrue {
			if !ddb.Status.TLS.Ready {
				ddb.Status.TLS.Ready = true
				ddb.Status.TLS.SecretName = cert.Spec.SecretName
				ddb.Status.TLS.Message = "Gateway TLS certificate ready"
				_ = r.Status().Update(ctx, ddb)
			}
			return nil
		}
	}
	ddb.Status.TLS.SecretName = cert.Spec.SecretName
	ddb.Status.TLS.Message = "Waiting for gateway TLS certificate to become ready"
	_ = r.Status().Update(ctx, ddb)
	return nil
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

	// Phase 1 TLS (skeleton): evaluate desired TLS mode and initialize status if needed.
	if err := r.reconcileGatewayTLS(ctx, documentdb); err != nil {
		log.Error(err, "TLS reconciliation failed (non-fatal)")
	}
	// Only create/manage the service if ExposeViaService is configured
	if documentdb.Spec.ExposeViaService.ServiceType != "" {
		serviceType := corev1.ServiceTypeClusterIP
		if documentdb.Spec.ExposeViaService.ServiceType == "LoadBalancer" {
			serviceType = corev1.ServiceTypeLoadBalancer // Public LoadBalancer service
		}

		// Define the Service for this DocumentDB instance
		ddbService := util.GetDocumentDBServiceDefinition(documentdb, req.Namespace, serviceType)

		// Check if the DocumentDB Service already exists for this instance
		foundService, err := util.GetOrCreateService(ctx, r.Client, ddbService)
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
		// Ensure plugin enabled and TLS secret parameter kept in sync once ready
		if documentdb.Status.TLS != nil && documentdb.Status.TLS.Ready && documentdb.Status.TLS.SecretName != "" {
			log.Info("Syncing TLS secret into CNPG Cluster plugin parameters", "secret", documentdb.Status.TLS.SecretName)
			updated := false
			for i := range currentCnpgCluster.Spec.Plugins {
				p := &currentCnpgCluster.Spec.Plugins[i]
				if p.Name == desiredCnpgCluster.Spec.Plugins[0].Name { // target our sidecar plugin
					if p.Enabled == nil || !*p.Enabled {
						trueVal := true
						p.Enabled = &trueVal
						updated = true
						log.Info("Enabled sidecar plugin")
					}
					if p.Parameters == nil {
						p.Parameters = map[string]string{}
					}
					currentVal := p.Parameters["gatewayTLSSecret"]
					if currentVal != documentdb.Status.TLS.SecretName {
						p.Parameters["gatewayTLSSecret"] = documentdb.Status.TLS.SecretName
						updated = true
						log.Info("Updated gatewayTLSSecret parameter", "old", currentVal, "new", documentdb.Status.TLS.SecretName)
					}
				}
			}
			if updated {
				if currentCnpgCluster.Annotations == nil {
					currentCnpgCluster.Annotations = map[string]string{}
				}
				currentCnpgCluster.Annotations["documentdb.microsoft.com/gateway-tls-rev"] = time.Now().Format(time.RFC3339Nano)
				if err := r.Client.Update(ctx, currentCnpgCluster); err == nil {
					log.Info("Patched CNPG Cluster with TLS settings; requeueing for pod update")
					return ctrl.Result{RequeueAfter: RequeueAfterShort}, nil
				} else {
					log.Error(err, "Failed to update CNPG Cluster with TLS settings")
				}
			}
		}
		// Update status connection string
		if documentDbServiceIp != "" {
			if documentdb.Status.TLS != nil && documentdb.Status.TLS.Ready {
				documentdb.Status.ConnectionString = util.GenerateSecureConnectionString(documentdb, documentDbServiceIp)
			} else {
				documentdb.Status.ConnectionString = util.GenerateConnectionString(documentdb, documentDbServiceIp)
			}
		}
		if err := r.Status().Update(ctx, documentdb); err != nil {
			log.Error(err, "Failed to update DocumentDB status and connection string")
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
		// React to cert-manager resource changes relevant to TLS readiness
		Owns(&cmapi.Certificate{}).
		Owns(&cmapi.Issuer{}).
		Named("documentdb-controller").
		Complete(r)
}
