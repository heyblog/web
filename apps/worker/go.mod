module heyblog-worker

go 1.27.2

require (
	github.com/google/uuid v1.6.0
	heyblog.local/edgerequest v0.0.0
	heyblog.local/taskprotocol v0.0.0
)

replace heyblog.local/edgerequest => ../../packages/go/edgerequest
replace heyblog.local/taskprotocol => ../../packages/go/taskprotocol
