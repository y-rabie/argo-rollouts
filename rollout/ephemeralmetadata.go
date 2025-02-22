package rollout

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	replicasetutil "github.com/argoproj/argo-rollouts/utils/replicaset"
)

// DefaultEphemeralMetadataThreads is the default number of worker threads to run when reconciling ephemeral metadata
const DefaultEphemeralMetadataThreads = 10

// reconcileEphemeralMetadata syncs canary/stable ephemeral metadata to ReplicaSets and pods
func (c *rolloutContext) reconcileEphemeralMetadata() error {
	ctx := context.TODO()
	var newMetadata, stableMetadata *v1alpha1.PodTemplateMetadata
	if c.rollout.Spec.Strategy.Canary != nil {
		newMetadata = c.rollout.Spec.Strategy.Canary.CanaryMetadata
		stableMetadata = c.rollout.Spec.Strategy.Canary.StableMetadata
	} else if c.rollout.Spec.Strategy.BlueGreen != nil {
		newMetadata = c.rollout.Spec.Strategy.BlueGreen.PreviewMetadata
		stableMetadata = c.rollout.Spec.Strategy.BlueGreen.ActiveMetadata
	} else {
		return nil
	}
	fullyRolledOut := c.rollout.Status.StableRS == "" || c.rollout.Status.StableRS == replicasetutil.GetPodTemplateHash(c.newRS)

	if fullyRolledOut {
		// We are in a steady-state (fully rolled out). newRS is the stableRS. there is no longer a canary
		err := c.syncEphemeralMetadata(ctx, c.newRS, stableMetadata)
		if err != nil {
			return err
		}
	} else {
		// we are in a upgrading state. newRS is a canary
		err := c.syncEphemeralMetadata(ctx, c.newRS, newMetadata)
		if err != nil {
			return err
		}
		// sync stable metadata to the stable rs
		err = c.syncEphemeralMetadata(ctx, c.stableRS, stableMetadata)
		if err != nil {
			return err
		}
	}

	// Iterate all other ReplicaSets and verify we don't have injected metadata for them
	for _, rs := range c.otherRSs {
		err := c.syncEphemeralMetadata(ctx, rs, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *rolloutContext) syncEphemeralMetadata(ctx context.Context, rs *appsv1.ReplicaSet, podMetadata *v1alpha1.PodTemplateMetadata) error {
	if rs == nil {
		return nil
	}
	modifiedRS, modified := replicasetutil.SyncReplicaSetEphemeralPodMetadata(rs, podMetadata)
	if !modified {
		return nil
	}

	// Used to access old ephemeral data when updating pods below
	originalRSCopy := rs.DeepCopy()

	// Order of the following two steps is important for race condition
	// First update replicasets, then pods owned by it.
	// So that any replicas created in the interim between the two steps are using the new updated version.
	// 1. Update ReplicaSet so that any new pods it creates will have the metadata
	rs, err := c.updateReplicaSet(ctx, modifiedRS)
	if err != nil {
		c.log.Infof("failed to sync ephemeral metadata %v to ReplicaSet %s: %v", podMetadata, originalRSCopy.Name, err)
		return fmt.Errorf("failed to sync ephemeral metadata: %w", err)
	}
	c.log.Infof("synced ephemeral metadata %v to ReplicaSet %s", podMetadata, rs.Name)

	// 2. Sync ephemeral metadata to pods
	pods, err := replicasetutil.GetPodsOwnedByReplicaSet(ctx, c.kubeclientset, rs)
	if err != nil {
		return err
	}
	existingPodMetadata := replicasetutil.ParseExistingPodMetadata(originalRSCopy)

	var eg errgroup.Group
	eg.SetLimit(c.ephemeralMetadataThreads)

	for _, pod := range pods {
		eg.Go(func() error {
			newPodObjectMeta, podModified := replicasetutil.SyncEphemeralPodMetadata(&pod.ObjectMeta, existingPodMetadata, podMetadata)
			if podModified {
				pod.ObjectMeta = *newPodObjectMeta
				_, err = c.kubeclientset.CoreV1().Pods(pod.Namespace).Update(ctx, pod, metav1.UpdateOptions{})
				if err != nil {
					return err
				}
				c.log.Infof("synced ephemeral metadata %v to Pod %s", podMetadata, pod.Name)
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}
