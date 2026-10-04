package gateway

type Access int

const (
	Public Access = iota
	Admin
)

type Route struct {
	Pattern  string
	Upstream string
	Access   Access
}

var Routes = []Route{
	{"GET /v1/presentation", "content", Public},
	{"GET /v1/sections", "content", Admin},
	{"POST /v1/sections", "content", Admin},
	{"GET /v1/sections/{id}", "content", Admin},
	{"PUT /v1/sections/{id}/draft", "content", Admin},
	{"POST /v1/sections/{id}/publish", "content", Admin},
	{"GET /v1/sections/{id}/versions", "content", Admin},
	{"POST /v1/sections/{id}/rollback", "content", Admin},

	{"GET /v1/questions", "interaction", Public},
	{"POST /v1/questions", "interaction", Public},
	{"POST /v1/questions/{id}/upvotes", "interaction", Public},
	{"POST /v1/questions/{id}/answer", "interaction", Admin},

	{"POST /v1/sessions", "realtime", Admin},
	{"GET /v1/sessions/{id}", "realtime", Public},
	{"PUT /v1/sessions/{id}/slide", "realtime", Admin},
}
