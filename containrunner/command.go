package containrunner

import (
	"fmt"
	"github.com/nu7hatch/gouuid"
	"reflect"
	"runtime"
	"sync"
)

// Structure for a single command execution. Invoke* functions return this and this
// can be used to Wait() for the command to end.
type Command struct {
	Id        string
	Name      string
	completed chan error
}

type CommandController struct {
	lock     sync.Mutex
	commands map[string]*Command
}

type CommandFunction func(interface{}) error

// Invokes function f with generic interface{} argument. Command is given a name via the reflection/runtime api.
//
// Returns *Command which can be used to Wait() until the command has executed.
func (cc *CommandController) Invoke(f CommandFunction, argument interface{}) (*Command, error) {
	cc.lock.Lock()
	if cc.commands == nil {
		cc.commands = make(map[string]*Command)
	}
	cc.lock.Unlock()

	return cc.InvokeNamed(runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name(), f, argument)
}

// Invokes function f with generic interface{} argument. Command is given a name which can be used to check
// if there's at least one other command still running with the same name.
//
// Returns *Command which can be used to Wait() until the command has executed.
func (cc *CommandController) InvokeNamed(name string, f CommandFunction, argument interface{}) (*Command, error) {
	cc.lock.Lock()
	if cc.commands == nil {
		cc.commands = make(map[string]*Command)
	}
	cc.lock.Unlock()

	command, err := cc.command(name, f, argument)

	return command, err
}

// Invokes function f with generic interface{} argument only if there is no another command running with
// the same name. Command is given a name which can be used to check if there's at least one other command
// still running with the same name.
//
// Returns *Command which can be used to Wait() until the command has executed.
func (cc *CommandController) InvokeIfNotAlreadyRunning(name string, f CommandFunction, argument interface{}) (*Command, error) {
	cc.lock.Lock()
	if cc.commands == nil {
		cc.commands = make(map[string]*Command)
	}
	cc.lock.Unlock()

	if cc.IsRunning(name) == false {
		command, err := cc.command(name, f, argument)
		return command, err
	}

	return nil, nil

}

// This function does not use locks. Locking for the CommandController.lock must be done by the caller.
func (cc *CommandController) command(name string, f CommandFunction, argument interface{}) (*Command, error) {
	command := new(Command)
	id, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}
	command.Id = id.String()
	command.Name = name
	command.completed = make(chan error)

	cc.lock.Lock()
	cc.commands[command.Id] = command
	cc.lock.Unlock()

	fmt.Printf("Going to execute command %s\n", name)
	go func() {
		err := f(argument)
		cc.lock.Lock()
		delete(cc.commands, command.Id)
		cc.lock.Unlock()
		command.completed <- err
	}()

	return command, nil
}

// Checks if there's at least one command running with the name.
func (cc *CommandController) IsRunning(name string) bool {
	cc.lock.Lock()
	defer cc.lock.Unlock()

	if cc.commands == nil {
		cc.commands = make(map[string]*Command)
	}

	for _, command := range cc.commands {
		if command.Name == name {
			return true
		}
	}
	return false
}

// Waits until command has finished running and returns the error value with it.
func (c *Command) Wait() error {
	return <-c.completed
}
