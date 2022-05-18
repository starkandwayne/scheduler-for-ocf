package ops

import (
	"github.com/ess/dry"

	"github.com/starkandwayne/scheduler-for-ocf/core"
)

func ValidateAppGUID(raw dry.Value) dry.Result {
	input := core.Inputify(raw)

	appGUID := input.Context.QueryParam("app_guid")
	if appGUID == "" {
		input.Services.Logger.Error(
			"ops.validate-app-guid",
			"app GUID cannot be blank",
		)

		return dry.Failure("no-app-guid")
	}

	input.Data["appGUID"] = appGUID

	return dry.Success(input)
}