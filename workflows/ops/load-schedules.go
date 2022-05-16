package ops

import (
	"github.com/ess/dry"

	"github.com/starkandwayne/scheduler-for-ocf/core"
)

func LoadSchedules(raw dry.Value) dry.Result {
	input := core.Inputify(raw)
	executable := input.Executable

	input.Schedules = input.Services.Schedules.ByRef(
		executable.RefType(),
		executable.RefGUID(),
	)

	return dry.Success(input)
}
