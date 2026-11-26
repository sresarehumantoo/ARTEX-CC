package tools

// RegisterAll registers every tool group with the given registry and engagement context.
func RegisterAll(reg *Registry, store Store, taskID, explorationID int64) {
	RegisterAssetTools(reg, store, taskID)
	RegisterExplorationTools(reg, store, explorationID)
	RegisterFindingTools(reg, store, taskID, explorationID)
	RegisterCredentialTools(reg, store, taskID)
	RegisterScopeTools(reg, store, taskID)
	RegisterReportTools(reg, store, taskID, explorationID)
	RegisterEngagementTools(reg, store, taskID, explorationID)
	RegisterEnrichTools(reg, store, taskID)
	RegisterActivityTools(reg, store, explorationID)
	RegisterIntentTools(reg, store, explorationID)
	RegisterChangeTools(reg, store, taskID, explorationID)
	RegisterTargetChangeTools(reg, store, explorationID)
	RegisterExportTools(reg, store, taskID, explorationID)
	RegisterIngestTools(reg, store, taskID, explorationID)
}
