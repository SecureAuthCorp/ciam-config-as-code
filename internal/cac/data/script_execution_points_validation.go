package data

import (
	"github.com/cloudentity/acp-client-go/clients/hub/models"
)

// workaround to pass too strict generated client validation when someone is trying to delete script execution points
func allowToDeleteScriptExecutionPoints(ts *models.TreeServer) {
	if ts.ScriptExecutionPoints == nil {
		return
	}

	for typeID, typee := range ts.ScriptExecutionPoints {
		for scriptID, script := range typee {
			// empty scriptID means that script is being deleted
			if script.ScriptID == "" {
				// scriptID is required, so we need to set it to some value to pass validation
				script.ScriptID = "[DELETED]"
				typee[scriptID] = script
			}
		}
		ts.ScriptExecutionPoints[typeID] = typee
	}
}
