// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package reconciler

import (
	"context"
	"fmt"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	"github.com/cloudnative-pg/cnpg-i-machinery/pkg/pluginhelper/common"
	"github.com/cloudnative-pg/machinery/pkg/log"
	"github.com/documentdb/cnpg-i-wal-replica/internal/config"
	"github.com/documentdb/cnpg-i-wal-replica/internal/k8sclient"
	"github.com/documentdb/cnpg-i-wal-replica/pkg/metadata"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func CreateWalReplica(
	ctx context.Context,
	cluster *cnpgv1.Cluster,
) error {
	logger := log.FromContext(ctx).WithName("CreateWalReplica")

	if !IsPrimaryCluster(cluster) {
		logger.Info("Cluster is not a primary, skipping wal replica creation", "cluster", cluster.Name)
		return nil
	}

	// Build Deployment name unique per cluster
	deploymentName := fmt.Sprintf("%s-wal-receiver", cluster.Name)
	namespace := cluster.Namespace
	client := k8sclient.MustGet()

	helper := common.NewPlugin(
		*cluster,
		metadata.PluginName,
	)

	configuration := config.FromParameters(helper)

	// TODO remove this once the operator functions are fixed
	configuration.ApplyDefaults(cluster)

	walDir := configuration.WalDirectory
	cmd := []string{
		"pg_receivewal", // TODO what do we do if it's not on the path?
		"--slot", "wal_replica",
		"--compress", "0",
		"--directory", walDir,
		"--dbname", GetConnectionString(configuration.ReplicationHost),
	}

	// TODO have a real check here
	if true {
		cmd = append(cmd, "--verbose")
	}

	// Add synchronous flag if requested
	if configuration.Synchronous == config.SynchronousActive {
		cmd = append(cmd, "--synchronous")
	}

	// Needs a PVC to store the wal data
	existingPVC := &corev1.PersistentVolumeClaim{}
	err := client.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: namespace}, existingPVC)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("WAL replica PVC not found. Creating a new WAL replica PVC")

		walReplicaPVC := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deploymentName,
				Namespace: namespace,
				Labels: map[string]string{
					"app":             deploymentName,
					"cnpg.io/cluster": cluster.Name,
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: cluster.APIVersion,
						Kind:       cluster.Kind,
						Name:       cluster.Name,
						UID:        cluster.UID,
					},
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("10Gi"),
					},
				},
			},
		}

		err = client.Create(ctx, walReplicaPVC)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	// Create replica slot

	// Create or patch Deployment
	existing := &appsv1.Deployment{}
	err = client.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: namespace}, existing)
	if err != nil {
		dep := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deploymentName,
				Namespace: namespace,
				Labels: map[string]string{
					"app":             deploymentName,
					"cnpg.io/cluster": cluster.Name,
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: cluster.APIVersion,
						Kind:       cluster.Kind,
						Name:       cluster.Name,
						UID:        cluster.UID,
					},
				},
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": deploymentName}},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": deploymentName}},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "wal-receiver",
							Image: configuration.Image,
							Args:  cmd,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      deploymentName,
									MountPath: walDir,
								},
								{
									Name:      "ca",
									MountPath: "/var/lib/postgresql/rootcert",
									ReadOnly:  true,
								},
								{
									Name:      "tls",
									MountPath: "/var/lib/postgresql/cert",
									ReadOnly:  true,
								},
							},
						}},
						Volumes: []corev1.Volume{
							{
								Name: deploymentName,
								VolumeSource: corev1.VolumeSource{
									PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: deploymentName,
									},
								},
							},
							{
								Name: "ca",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName:  cluster.Status.Certificates.ServerCASecret,
										DefaultMode: int32Ptr(0600),
									},
								},
							},
							{
								Name: "tls",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName:  cluster.Status.Certificates.ReplicationTLSSecret,
										DefaultMode: int32Ptr(0600),
									},
								},
							},
						},
						SecurityContext: &corev1.PodSecurityContext{
							RunAsUser:  int64Ptr(105),
							RunAsGroup: int64Ptr(103),
							FSGroup:    int64Ptr(103),
						},
						RestartPolicy: corev1.RestartPolicyAlways,
					},
				},
			},
		}
		if createErr := client.Create(ctx, dep); createErr != nil {
			logger.Error(createErr, "creating wal receiver deployment")
			return createErr
		}
		logger.Info("created wal receiver deployment", "name", deploymentName)
	} else {
		// TODO handle patch
	}

	return nil
}

func GetConnectionString(host string) string {
	return fmt.Sprintf("postgres://%s@%s/postgres?sslmode=verify-full&sslrootcert=%s&sslcert=%s&sslkey=%s",
		"streaming_replica", // user
		host,
		"/var/lib/postgresql/rootcert/ca.crt", // root cert
		"/var/lib/postgresql/cert/tls.crt",    // cert
		"/var/lib/postgresql/cert/tls.key")    // key
}
func int64Ptr(i int64) *int64 {
	return &i
}
func int32Ptr(i int32) *int32 {
	return &i
}

func IsPrimaryCluster(cluster *cnpgv1.Cluster) bool {
	// TODO implement
	return true
}
