package logging

// Typed structures for domains
type AuthDomain struct {
	*LogDomain
	// Direct actions
	InvalidCredentials *LogAction
	SessionExpired     *LogAction

	// Sub-domains
	Login    *AuthLoginDomain
	Register *AuthRegisterDomain
	JWT      *AuthJWTDomain
}

type ServerDomain struct {
	*LogDomain
	Started     *LogAction
	Stopped     *LogAction
	Restarted   *LogAction
	Deleted     *LogAction
	CommandExec *LogAction
}

type APIDomain struct {
	*LogDomain
	Started   *LogAction
	Stopped   *LogAction
	Restarted *LogAction
	Deleted   *LogAction
}

type DBDomain struct {
	*LogDomain
	Connected    *LogAction
	Disconnected *LogAction
	Failed       *LogAction
}

// Sub-domain structures

type AuthLoginDomain struct {
	*LogDomain
	Success *LogAction
	Failed  *LogAction
}

type AuthRegisterDomain struct {
	*LogDomain
	Success *LogAction
	Failed  *LogAction
}

type AuthJWTDomain struct {
	*LogDomain
	Created *LogAction
	Expired *LogAction
	Invalid *LogAction
	Revoked *LogAction
}
