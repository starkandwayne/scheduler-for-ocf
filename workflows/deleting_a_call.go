package workflows

import (
	"github.com/ess/dry"

	"github.com/starkandwayne/scheduler-for-ocf/workflows/ops"
)

var DeletingACall = dry.NewTransaction(
	ops.VerifyAuth,
	ops.LoadCall,
	ops.LoadSchedules,
	ops.DeleteScheduleCollection,
	ops.DeleteCall,
)
