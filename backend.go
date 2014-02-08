package libcontainer

type Backend interface {
	Exec(container *Container) (pid int, err error)                 // exec the container in a new namespace
	ExecIn(container *Container, cmd *Command) (pid int, err error) // exec a new command in an existing container
}
