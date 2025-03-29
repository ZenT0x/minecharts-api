package logging

// Exported variables
var (
	Auth   *AuthDomain
	Server *ServerDomain
	API    *APIDomain
	DB     *DBDomain
)

// Initialization of domains
func InitStructuredLogging() {
	// Initialize Auth domain
	Auth = &AuthDomain{
		LogDomain: Domain("Auth"),
	}
	Auth.InvalidCredentials = Auth.LogDomain.Action("InvalidCredentials")
	Auth.SessionExpired = Auth.LogDomain.Action("SessionExpired")

	// Initialize Login subdomain
	Auth.Login = &AuthLoginDomain{
		LogDomain: Auth.LogDomain.SubDomain("Login"),
	}
	Auth.Login.Success = Auth.Login.LogDomain.Action("Success")
	Auth.Login.Failed = Auth.Login.LogDomain.Action("Failed")

	// Initialize Register subdomain
	Auth.Register = &AuthRegisterDomain{
		LogDomain: Auth.LogDomain.SubDomain("Register"),
	}
	Auth.Register.Success = Auth.Register.LogDomain.Action("Success")
	Auth.Register.Failed = Auth.Register.LogDomain.Action("Failed")

	// Initialize JWT subdomain
	Auth.JWT = &AuthJWTDomain{
		LogDomain: Auth.LogDomain.SubDomain("JWT"),
	}
	Auth.JWT.Created = Auth.JWT.LogDomain.Action("Created")
	Auth.JWT.Expired = Auth.JWT.LogDomain.Action("Expired")
	Auth.JWT.Invalid = Auth.JWT.LogDomain.Action("Invalid")
	Auth.JWT.Revoked = Auth.JWT.LogDomain.Action("Revoked")

	// Initialize API domain
	API = &APIDomain{
		LogDomain: Domain("API"),
	}
	API.Started = API.LogDomain.Action("Started")
	API.Stopped = API.LogDomain.Action("Stopped")
	API.Restarted = API.LogDomain.Action("Restarted")
	API.Deleted = API.LogDomain.Action("Deleted")

	// Initialize Database domain
	DB = &DBDomain{
		LogDomain: Domain("Database"),
	}
	DB.Connected = DB.LogDomain.Action("Connected")
	DB.Disconnected = DB.LogDomain.Action("Disconnected")
	DB.Failed = DB.LogDomain.Action("Failed")

	// Initialize Server domain
	Server = &ServerDomain{
		LogDomain: Domain("Server"),
	}
	Server.Started = Server.LogDomain.Action("Started")
	Server.Stopped = Server.LogDomain.Action("Stopped")
	Server.Restarted = Server.LogDomain.Action("Restarted")
	Server.Deleted = Server.LogDomain.Action("Deleted")
	Server.CommandExec = Server.LogDomain.Action("CommandExec")

	// And so on for other domains...
}
