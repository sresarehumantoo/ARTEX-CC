package tools

func RegisterAll(reg *Registry, store Store, taskID, explorationID int64) {
	RegisterAssetTools(reg, store, taskID)
	RegisterExplorationTools(reg, store, explorationID)
	RegisterFindingTools(reg, store, taskID, explorationID)
	RegisterCredentialTools(reg, store, taskID)
	RegisterScopeTools(reg, store, taskID)
	RegisterReportTools(reg, store, taskID, explorationID)
	RegisterEngagementTools(reg, store, taskID, explorationID)
}
