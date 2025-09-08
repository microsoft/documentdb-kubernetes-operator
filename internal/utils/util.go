// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package util

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dbpreview "github.com/microsoft/documentdb-operator/api/preview"
)

// DeleteService deletes a Service for a given DocumentDB instance
func DeleteService(ctx context.Context, c client.Client, serviceName, namespace string) error {
	service := &corev1.Service{}
	err := c.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: namespace}, service)
	if err == nil {
		err = c.Delete(ctx, service)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetDocumentDBServiceDefinition returns the LoadBalancer Service definition for a given DocumentDB instance
func GetDocumentDBServiceDefinition(documentdb *dbpreview.DocumentDB, namespace string, serviceType corev1.ServiceType) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DOCUMENTDB_SERVICE_PREFIX + documentdb.Name, // Unique service name
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				LABEL_APP:          documentdb.Name,
				LABEL_REPLICA_TYPE: "primary", // Service forwards traffic to primary replicas
			},
			Ports: []corev1.ServicePort{
				{Name: "gateway", Protocol: corev1.ProtocolTCP, Port: GetPortFor(GATEWAY_PORT), TargetPort: intstr.FromInt(int(GetPortFor(GATEWAY_PORT)))},
			},
			Type: serviceType,
		},
	}
}

// EnsureServiceIP ensures that the Service has an IP assigned and returns it, or returns an error if not available
func EnsureServiceIP(ctx context.Context, service *corev1.Service) (string, error) {
	if service == nil {
		return "", fmt.Errorf("service is nil")
	}

	// For ClusterIP services, return the ClusterIP directly
	if service.Spec.Type == corev1.ServiceTypeClusterIP {
		if service.Spec.ClusterIP != "" && service.Spec.ClusterIP != "None" {
			return service.Spec.ClusterIP, nil
		}
		return "", fmt.Errorf("ClusterIP not assigned")
	}

	// For LoadBalancer services, wait for external IP to be assigned
	if service.Spec.Type == corev1.ServiceTypeLoadBalancer {
		retries := 5
		for i := 0; i < retries; i++ {
			if len(service.Status.LoadBalancer.Ingress) > 0 && service.Status.LoadBalancer.Ingress[0].IP != "" {
				return service.Status.LoadBalancer.Ingress[0].IP, nil
			}
			time.Sleep(time.Second * 10)
		}
		return "", fmt.Errorf("LoadBalancer IP not assigned after %d retries", retries)
	}

	return "", fmt.Errorf("unsupported service type: %s", service.Spec.Type)
}

// GetOrCreateService checks if the Service already exists, and creates it if not.
func GetOrCreateService(ctx context.Context, c client.Client, service *corev1.Service) (*corev1.Service, error) {
	log := log.FromContext(ctx)
	foundService := &corev1.Service{}
	err := c.Get(ctx, types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, foundService)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Service not found. Creating a new one: ", "Service.Namespace", service.Namespace, "Service.Name", service.Name)
			if err := c.Create(ctx, service); err != nil && !errors.IsAlreadyExists(err) {
				return nil, err
			}
			// Refresh foundService after creating the new Service
			time.Sleep(10 * time.Second)
			if err := c.Get(ctx, types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, foundService); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	return foundService, nil
}

func GetPortFor(name string) int32 {
	switch name {
	case POSTGRES_PORT:
		return getEnvAsInt32(POSTGRES_PORT, 5432)
	case SIDECAR_PORT:
		return getEnvAsInt32(SIDECAR_PORT, 8445)
	case GATEWAY_PORT:
		return getEnvAsInt32(GATEWAY_PORT, 10260)
	default:
		return 0
	}
}

func getEnvAsInt32(name string, defaultVal int) int32 {
	if value, exists := os.LookupEnv(name); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return int32(intValue)
		} else {
			log.FromContext(context.Background()).Error(err, "Invalid integer value for environment variable", "name", name, "value", value)
		}
	}
	return int32(defaultVal)
}

// CreateRole creates a Role with the given name in the specified namespace
func CreateRole(ctx context.Context, c client.Client, name, namespace string, rules []rbacv1.PolicyRule) error {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Rules: rules,
	}
	foundRole := &rbacv1.Role{}
	err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, foundRole)
	if err == nil {
		return nil // Role already exists
	}
	if errors.IsNotFound(err) {
		if err := c.Create(ctx, role); err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	} else {
		return err
	}
	return nil
}

// CreateServiceAccount creates a ServiceAccount with the given name in the specified namespace
func CreateServiceAccount(ctx context.Context, c client.Client, name, namespace string) error {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	foundServiceAccount := &corev1.ServiceAccount{}
	err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, foundServiceAccount)
	if err == nil {
		return nil // ServiceAccount already exists
	}
	if errors.IsNotFound(err) {
		if err := c.Create(ctx, serviceAccount); err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	} else {
		return err
	}
	return nil
}

// CreateRoleBinding creates a RoleBinding with the given name in the specified namespace
func CreateRoleBinding(ctx context.Context, c client.Client, name, namespace string) error {
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      name,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     name,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	foundRoleBinding := &rbacv1.RoleBinding{}
	err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, foundRoleBinding)
	if err == nil {
		return nil // RoleBinding already exists
	}
	if errors.IsNotFound(err) {
		if err := c.Create(ctx, roleBinding); err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	} else {
		return err
	}
	return nil
}

// DeleteServiceAccount deletes the ServiceAccount with the given name in the specified namespace
func DeleteServiceAccount(ctx context.Context, c client.Client, name, namespace string) error {
	serviceAccount := &corev1.ServiceAccount{}
	err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, serviceAccount)
	if err == nil {
		if err := c.Delete(ctx, serviceAccount); err != nil {
			return err
		}
	}
	return nil
}

// DeleteRole deletes the Role with the given name in the specified namespace
func DeleteRole(ctx context.Context, c client.Client, name, namespace string) error {
	role := &rbacv1.Role{}
	err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, role)
	if err == nil {
		if err := c.Delete(ctx, role); err != nil {
			return err
		}
	}
	return nil
}

// DeleteRoleBinding deletes the RoleBinding with the given name in the specified namespace
func DeleteRoleBinding(ctx context.Context, c client.Client, name, namespace string) error {
	roleBinding := &rbacv1.RoleBinding{}
	err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, roleBinding)
	if err == nil {
		if err := c.Delete(ctx, roleBinding); err != nil {
			return err
		}
	}
	return nil
}

// GenerateConnectionString returns a MongoDB connection string for the DocumentDB instance
func GenerateConnectionString(documentdb *dbpreview.DocumentDB, serviceIp string) string {
	secretName := documentdb.Spec.DocumentDbCredentialSecret
	if secretName == "" {
		secretName = DEFAULT_DOCUMENTDB_CREDENTIALS_SECRET
	}
	return fmt.Sprintf("mongodb://$(kubectl get secret %s -n %s -o jsonpath='{.data.username}' | base64 -d):$(kubectl get secret %s -n %s -o jsonpath='{.data.password}' | base64 -d)@%s:%d/?directConnection=true&authMechanism=SCRAM-SHA-256&tls=true&tlsAllowInvalidCertificates=true&replicaSet=rs0", secretName, documentdb.Namespace, secretName, documentdb.Namespace, serviceIp, GetPortFor(GATEWAY_PORT))
}

// GetGatewayImageForDocumentDB returns the gateway image for a DocumentDB instance.
// Priority: spec.gatewayImage > spec.documentDBVersion > env.DOCUMENTDB_GATEWAY_IMAGE > env.DOCUMENTDB_VERSION > default
func GetGatewayImageForDocumentDB(documentdb *dbpreview.DocumentDB) string {
	if documentdb.Spec.GatewayImage != "" {
		return documentdb.Spec.GatewayImage
	}

	// Use spec-level documentDBVersion if set
	if documentdb.Spec.DocumentDBVersion != "" {
		return fmt.Sprintf("%s:%s", DOCUMENTDB_IMAGE_REPOSITORY, documentdb.Spec.DocumentDBVersion)
	}

	// Use environment variable if set (for documentDbVersion)
	if gatewayImage := os.Getenv("DOCUMENTDB_GATEWAY_IMAGE"); gatewayImage != "" {
		return gatewayImage
	}

	// Use global documentDbVersion if set
	if version := os.Getenv(DOCUMENTDB_VERSION_ENV); version != "" {
		return fmt.Sprintf("%s:%s", DOCUMENTDB_IMAGE_REPOSITORY, version)
	}

	// Fall back to default
	return DEFAULT_GATEWAY_IMAGE
}

// GetDocumentDBImageForInstance returns the documentdb engine image.
// Priority: spec.documentDBImage > spec.documentDBVersion > env.DOCUMENTDB_IMAGE > env.DOCUMENTDB_VERSION > default
func GetDocumentDBImageForInstance(documentdb *dbpreview.DocumentDB) string {
	if documentdb.Spec.DocumentDBImage != "" {
		return documentdb.Spec.DocumentDBImage
	}

	// Use spec-level documentDBVersion if set
	if documentdb.Spec.DocumentDBVersion != "" {
		return fmt.Sprintf("%s:%s", DOCUMENTDB_IMAGE_REPOSITORY, documentdb.Spec.DocumentDBVersion)
	}

	// Use environment variable if set
	if dbImage := os.Getenv(COSMOSDB_IMAGE_ENV); dbImage != "" {
		return dbImage
	}

	// Use global documentDbVersion if set
	if version := os.Getenv(DOCUMENTDB_VERSION_ENV); version != "" {
		return fmt.Sprintf("%s:%s", DOCUMENTDB_IMAGE_REPOSITORY, version)
	}

	// Fall back to default
	return DEFAULT_DOCUMENTDB_IMAGE
}
