// Copyright 2026 Rubrik, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to
// deal in the Software without restriction, including without limitation the
// rights to use, copy, modify, merge, publish, distribute, sublicense, and/or
// sell copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
// FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
// DEALINGS IN THE SOFTWARE.

package provider

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/devops"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/hierarchy"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/sla"
)

// waitForAssignment waits for all objects to have the expected SLA directly assigned.
// For doNotProtect, expectedDomainID should be gqlsla.DoNotProtectSLAID.
// For protectWithSlaId, expectedDomainID should be the SLA domain ID string.
// For unassignment (noAssignment), expectedDomainID should be gqlsla.UnprotectedSLAID.
func waitForAssignment(ctx context.Context, client *polaris.Client, expectedDomainID string, objectIDs []uuid.UUID, workload hierarchy.Workload) error {
	startTime := time.Now()

	for {
		pending := make([]uuid.UUID, 0, len(objectIDs))

		for _, objectID := range objectIDs {
			obj, err := workaroundBrokenHierarchy(ctx, client, objectID, workload)
			if err != nil {
				return err
			}

			switch expectedDomainID {
			case gqlsla.UnprotectedSLAID:
				// For unprotected, we just need to wait until there's no direct assignment.
				if obj.SLAAssignment == gqlsla.Direct || obj.ConfiguredSLADomain.ID != gqlsla.UnprotectedSLAID {
					tflog.Debug(ctx, "waiting for assignment to be removed (sla_assignment != DIRECT)", map[string]any{
						"object_id":      objectID.String(),
						"configured_sla": obj.ConfiguredSLADomain.ID,
						"effective_sla":  obj.EffectiveSLADomain.ID,
						"elapsed":        time.Since(startTime).Truncate(time.Second).String(),
					})
					pending = append(pending, objectID)
				}
			case gqlsla.DoNotProtectSLAID:
				// For doNotProtect, we check effectiveSlaDomain because with
				// RETAIN_SNAPSHOTS, configuredSlaDomain keeps the previous SLA.
				if obj.EffectiveSLADomain.ID != gqlsla.DoNotProtectSLAID {
					tflog.Debug(ctx, "waiting for transition to DO_NOT_PROTECT", map[string]any{
						"object_id":      objectID.String(),
						"sla_assignment": string(obj.SLAAssignment),
						"configured_sla": obj.ConfiguredSLADomain.ID,
						"effective_sla":  obj.EffectiveSLADomain.ID,
						"elapsed":        time.Since(startTime).Truncate(time.Second).String(),
					})
					pending = append(pending, objectID)
				}
			default:
				// For other cases, check if the object has the expected SLA directly assigned.
				if obj.SLAAssignment != gqlsla.Direct || obj.ConfiguredSLADomain.ID != expectedDomainID {
					tflog.Debug(ctx, "waiting for SLA assignment", map[string]any{
						"object_id":      objectID.String(),
						"expected_sla":   expectedDomainID,
						"sla_assignment": string(obj.SLAAssignment),
						"configured_sla": obj.ConfiguredSLADomain.ID,
						"effective_sla":  obj.EffectiveSLADomain.ID,
						"elapsed":        time.Since(startTime).Truncate(time.Second).String(),
					})
					pending = append(pending, objectID)
				}
			}
		}

		if len(pending) == 0 {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

// waitForAssignmentRemoval waits for objects to no longer have the specified SLA
// directly assigned. Unlike waitForAssignment, this function accepts that objects
// may be reassigned to a different SLA (not just unprotected) as a valid completion
// state. This prevents race conditions when Terraform replaces resources by deleting
// the old assignment and creating new ones concurrently.
//
// Parameters:
//   - currentSLADomainID: The SLA domain ID being removed (can be a UUID string,
//     gqlsla.DoNotProtectSLAID, or gqlsla.UnprotectedSLAID)
//   - objectIDs: The objects to check
//
// The function completes successfully when all objects either:
//  1. No longer have a direct assignment (inherited or unprotected), OR
//  2. Have been directly assigned to a different SLA domain
func waitForAssignmentRemoval(ctx context.Context, client *polaris.Client, currentSLADomainID string, objectIDs []uuid.UUID, workload hierarchy.Workload) error {
	startTime := time.Now()

	for {
		pending := make([]uuid.UUID, 0, len(objectIDs))

		for _, objectID := range objectIDs {
			obj, err := workaroundBrokenHierarchy(ctx, client, objectID, workload)
			if err != nil {
				return err
			}

			// Check if the object still has the old SLA directly assigned.
			stillHasOldAssignment := false

			switch currentSLADomainID {
			case gqlsla.DoNotProtectSLAID:
				// For doNotProtect, check if effective SLA is still DO_NOT_PROTECT with direct assignment.
				if obj.SLAAssignment == gqlsla.Direct && obj.EffectiveSLADomain.ID == gqlsla.DoNotProtectSLAID {
					stillHasOldAssignment = true
				}
			case gqlsla.UnprotectedSLAID:
				// For unprotected, check if it's still unprotected with direct assignment.
				// Note: This case is unlikely in practice since unprotected means no direct assignment.
				if obj.SLAAssignment == gqlsla.Direct && obj.ConfiguredSLADomain.ID == gqlsla.UnprotectedSLAID {
					stillHasOldAssignment = true
				}
			default:
				// For regular SLA domains, check if the configured SLA matches the old one.
				if obj.SLAAssignment == gqlsla.Direct && obj.ConfiguredSLADomain.ID == currentSLADomainID {
					stillHasOldAssignment = true
				}
			}

			if stillHasOldAssignment {
				tflog.Debug(ctx, "waiting for old SLA assignment to be removed or replaced", map[string]any{
					"object_id":      objectID.String(),
					"old_sla":        currentSLADomainID,
					"sla_assignment": string(obj.SLAAssignment),
					"configured_sla": obj.ConfiguredSLADomain.ID,
					"effective_sla":  obj.EffectiveSLADomain.ID,
					"elapsed":        time.Since(startTime).Truncate(time.Second).String(),
				})
				pending = append(pending, objectID)
			} else {
				// Object no longer has the old assignment - either it's been reassigned
				// to a different SLA or it's now inherited/unprotected. Either way, our
				// delete operation is complete for this object.
				tflog.Debug(ctx, "old SLA assignment removed or replaced", map[string]any{
					"object_id":      objectID.String(),
					"old_sla":        currentSLADomainID,
					"sla_assignment": string(obj.SLAAssignment),
					"configured_sla": obj.ConfiguredSLADomain.ID,
					"effective_sla":  obj.EffectiveSLADomain.ID,
				})
			}
		}

		if len(pending) == 0 {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

func ptr[T any](v T) *T {
	return &v
}

// workaroundBrokenHierarchy looks up a hierarchy object by ID and workload.
// When the workload is WorkloadAllSubHierarchyType and the generic lookup
// fails, RSC may be returning an internal server error for object types that
// currently do not support the generic hierarchy endpoint (Azure DevOps
// projects, GitHub organizations, and GitHub repositories). In that case the
// function retries with the object-type-specific query for each of those three
// types and returns the first one that succeeds.
func workaroundBrokenHierarchy(ctx context.Context, client *polaris.Client, objectID uuid.UUID, workload hierarchy.Workload) (gqlsla.HierarchyObject, error) {
	obj, err := sla.Wrap(client).HierarchyObjectByIDAndWorkload(ctx, objectID, workload)

	// Workaround for broken hierarchy, Azure DevOps Projects, GitHub Orgs and
	// GitHub repositories all return an internal server error when querying
	// them with the generic call, retry with an object specific call.
	if err != nil && workload == hierarchy.WorkloadAllSubHierarchyType {
		tflog.Debug(ctx, "Object lookup resulted in an error, trying workaround", map[string]any{
			"object_id": objectID.String(),
		})

		adoProj, err := devops.Wrap(client).AzureProjectByID(ctx, objectID)
		if err == nil {
			return adoProj.HierarchyObject, nil
		}

		ghOrg, err := devops.Wrap(client).GitHubOrganizationByID(ctx, objectID)
		if err == nil {
			return ghOrg.HierarchyObject, nil
		}

		ghRepo, err := devops.Wrap(client).GitHubRepositoryByID(ctx, objectID)
		if err == nil {
			return ghRepo.HierarchyObject, nil
		}

		tflog.Debug(ctx, "Object lookup workaround did not solve the error", map[string]any{
			"object_id": objectID.String(),
		})
	}

	return obj, err
}
