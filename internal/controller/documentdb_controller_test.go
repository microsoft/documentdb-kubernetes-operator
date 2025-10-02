package controller

import (
	"context"
	"testing"
	"time"

	cmapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	dbpreview "github.com/microsoft/documentdb-operator/api/preview"
	util "github.com/microsoft/documentdb-operator/internal/utils"
)

// helper to build TLS reconciler with objects
func buildGatewayTLSReconciler(t *testing.T, objs ...runtime.Object) *GatewayTLSReconciler {
	scheme := runtime.NewScheme()
	require.NoError(t, dbpreview.AddToScheme(scheme))
	require.NoError(t, cmapi.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
	return &GatewayTLSReconciler{Client: c, Scheme: scheme}
}

func baseDocumentDB(name, ns string) *dbpreview.DocumentDB {
	return &dbpreview.DocumentDB{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: dbpreview.DocumentDBSpec{
			NodeCount:        1,
			InstancesPerNode: 1,
			Resource:         dbpreview.Resource{PvcSize: "1Gi"},
			DocumentDBImage:  "test-image",
			ExposeViaService: dbpreview.ExposeViaService{ServiceType: "ClusterIP"},
		},
	}
}

func TestEnsureProvidedSecret(t *testing.T) {
	ctx := context.Background()
	ddb := baseDocumentDB("ddb-prov", "default")
	ddb.Spec.TLS = &dbpreview.TLSConfiguration{Gateway: &dbpreview.GatewayTLS{Mode: "Provided", Provided: &dbpreview.ProvidedTLS{SecretName: "mycert"}}}
	// Secret missing first
	r := buildGatewayTLSReconciler(t, ddb)
	res, err := r.reconcileGatewayTLS(ctx, ddb)
	require.NoError(t, err)
	require.Equal(t, RequeueAfterShort, res.RequeueAfter)
	require.False(t, ddb.Status.TLS.Ready, "Should not be ready until secret exists")

	// Create secret with required keys then reconcile again
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "mycert", Namespace: "default"}, Data: map[string][]byte{"tls.crt": []byte("crt"), "tls.key": []byte("key")}}
	require.NoError(t, r.Client.Create(ctx, secret))
	res, err = r.reconcileGatewayTLS(ctx, ddb)
	require.NoError(t, err)
	require.Zero(t, res.RequeueAfter)
	require.True(t, ddb.Status.TLS.Ready, "Provided secret should mark TLS ready")
	require.Equal(t, "mycert", ddb.Status.TLS.SecretName)
}

func TestEnsureCertManagerManagedCert(t *testing.T) {
	ctx := context.Background()
	ddb := baseDocumentDB("ddb-cm", "default")
	ddb.Spec.TLS = &dbpreview.TLSConfiguration{Gateway: &dbpreview.GatewayTLS{Mode: "CertManager", CertManager: &dbpreview.CertManagerTLS{IssuerRef: dbpreview.IssuerRef{Name: "test-issuer", Kind: "Issuer"}, DNSNames: []string{"custom.example"}}}}
	ddb.Status.TLS = &dbpreview.TLSStatus{}
	issuer := &cmapi.Issuer{ObjectMeta: metav1.ObjectMeta{Name: "test-issuer", Namespace: "default"}, Spec: cmapi.IssuerSpec{IssuerConfig: cmapi.IssuerConfig{SelfSigned: &cmapi.SelfSignedIssuer{}}}}
	r := buildGatewayTLSReconciler(t, ddb, issuer)

	// Call certificate ensure twice to mimic reconcile loops
	res, err := r.reconcileGatewayTLS(ctx, ddb)
	require.NoError(t, err)
	require.Equal(t, RequeueAfterShort, res.RequeueAfter)
	res, err = r.reconcileGatewayTLS(ctx, ddb)
	require.NoError(t, err)
	require.Equal(t, RequeueAfterShort, res.RequeueAfter)

	cert := &cmapi.Certificate{}
	// fetch certificate (self-created by reconcile). If not found, run reconcile again once.
	require.NoError(t, r.Client.Get(ctx, types.NamespacedName{Name: "ddb-cm-gateway-cert", Namespace: "default"}, cert))
	// Debug: list all certificates to ensure store functioning
	certList := &cmapi.CertificateList{}
	_ = r.Client.List(ctx, certList)
	for _, c := range certList.Items {
		t.Logf("Found certificate: %s/%s secret=%s", c.Namespace, c.Name, c.Spec.SecretName)
	}
	require.Contains(t, cert.Spec.DNSNames, "custom.example")
	// Should include service DNS names
	serviceBase := util.DOCUMENTDB_SERVICE_PREFIX + ddb.Name
	require.Contains(t, cert.Spec.DNSNames, serviceBase)

	// Simulate readiness condition then invoke ensure again (mimic reconcile loop)
	cert.Status.Conditions = append(cert.Status.Conditions, cmapi.CertificateCondition{Type: cmapi.CertificateConditionReady, Status: cmmeta.ConditionTrue, LastTransitionTime: &metav1.Time{Time: time.Now()}})
	require.NoError(t, r.Client.Update(ctx, cert))
	res, err = r.reconcileGatewayTLS(ctx, ddb)
	require.NoError(t, err)
	require.Zero(t, res.RequeueAfter)
	require.True(t, ddb.Status.TLS.Ready, "Cert-manager managed cert should mark ready after condition true")
	require.NotEmpty(t, ddb.Status.TLS.SecretName)
}

func TestEnsureSelfSignedCert(t *testing.T) {
	ctx := context.Background()
	ddb := baseDocumentDB("ddb-ss", "default")
	ddb.Spec.TLS = &dbpreview.TLSConfiguration{Gateway: &dbpreview.GatewayTLS{Mode: "SelfSigned"}}
	ddb.Status.TLS = &dbpreview.TLSStatus{}
	r := buildGatewayTLSReconciler(t, ddb)

	// First call should create issuer and certificate
	res, err := r.reconcileGatewayTLS(ctx, ddb)
	require.NoError(t, err)
	require.Equal(t, RequeueAfterShort, res.RequeueAfter)

	// Certificate should exist
	cert := &cmapi.Certificate{}
	require.NoError(t, r.Client.Get(ctx, types.NamespacedName{Name: "ddb-ss-gateway-cert", Namespace: "default"}, cert))

	// Simulate ready condition and call again
	cert.Status.Conditions = append(cert.Status.Conditions, cmapi.CertificateCondition{Type: cmapi.CertificateConditionReady, Status: cmmeta.ConditionTrue, LastTransitionTime: &metav1.Time{Time: time.Now()}})
	require.NoError(t, r.Client.Update(ctx, cert))
	res, err = r.reconcileGatewayTLS(ctx, ddb)
	require.NoError(t, err)
	require.Zero(t, res.RequeueAfter)
	require.True(t, ddb.Status.TLS.Ready)
	require.NotEmpty(t, ddb.Status.TLS.SecretName)
}
