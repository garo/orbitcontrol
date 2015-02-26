package containrunner

import (
	"fmt"
	. "gopkg.in/check.v1"
)

type CommandSuite struct {
}

var _ = Suite(&CommandSuite{})

func (s *CommandSuite) TestInvokeCommand(c *C) {

	cc := CommandController{}

	called := false

	f := func(arguments interface{}) error {
		fmt.Printf("Function called\n")
		called = true

		return nil
	}

	command, err := cc.Invoke(f, nil)
	c.Assert(err, Equals, nil)

	fmt.Printf("UUID: %s, name: %s\n", command.Id, command.Name)

	err = command.Wait()
	c.Assert(err, Equals, nil)
	c.Assert(called, Equals, true)

}

func (s *CommandSuite) TestInvokeNamedCommand(c *C) {

	cc := CommandController{}

	f := func(arguments interface{}) error {
		return nil
	}

	command, err := cc.InvokeNamed("test", f, nil)
	c.Assert(err, Equals, nil)
	c.Assert(command.Name, Equals, "test")
}

func (s *CommandSuite) TestInvokeIfNotAlreadyRunning(c *C) {

	cc := CommandController{}

	cont := make(chan error, 10)

	f := func(arguments interface{}) error {
		return <-cont
	}

	command, err := cc.InvokeNamed("test", f, nil)
	c.Assert(command, Not(Equals), nil)
	c.Assert(err, Equals, nil)
	c.Assert(command.Name, Equals, "test")

	c.Assert(true, Equals, cc.IsRunning("test"))

	c2, err := cc.InvokeIfNotAlreadyRunning("test", f, nil)
	c.Assert(err, Equals, nil)
	c.Assert(c2, IsNil)

	cont <- nil
	command.Wait()
	c.Assert(false, Equals, cc.IsRunning("test"))

	command, err = cc.InvokeIfNotAlreadyRunning("test", f, nil)
	c.Assert(command, Not(IsNil))
	c.Assert(err, IsNil)
	c.Assert(true, Equals, cc.IsRunning("test"))

	cont <- nil
	command.Wait()
	c.Assert(false, Equals, cc.IsRunning("test"))

}
