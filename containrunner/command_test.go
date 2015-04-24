package containrunner

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestInvokeCommand(t *testing.T) {

	cc := CommandController{}

	called := false

	f := func(arguments interface{}) error {
		fmt.Printf("Function called\n")
		called = true

		return nil
	}

	command, err := cc.Invoke(f, nil)
	assert.Nil(t, err)

	fmt.Printf("UUID: %s, name: %s\n", command.Id, command.Name)

	err = command.Wait()
	fmt.Printf("command wait done. err: %+v\n", err)
	assert.Nil(t, err)
	assert.Equal(t, true, called)

}

func TestInvokeNamedCommand(t *testing.T) {

	cc := CommandController{}

	f := func(arguments interface{}) error {
		return nil
	}

	command, err := cc.InvokeNamed("test", f, nil)

	assert.Nil(t, err)
	assert.Equal(t, "test", command.Name)

}

func TestInvokeIfNotAlreadyRunning(t *testing.T) {

	cc := CommandController{}

	cont := make(chan error, 10)

	f := func(arguments interface{}) error {
		return <-cont
	}

	command, err := cc.InvokeNamed("test", f, nil)
	assert.NotNil(t, command)
	assert.Nil(t, err)
	assert.Equal(t, "test", command.Name)

	assert.Equal(t, true, cc.IsRunning("test"))

	c2, err := cc.InvokeIfNotAlreadyRunning("test", f, nil)
	assert.Nil(t, err)
	assert.Nil(t, c2)

	cont <- nil
	command.Wait()
	assert.Equal(t, false, cc.IsRunning("test"))

	command, err = cc.InvokeIfNotAlreadyRunning("test", f, nil)
	assert.NotNil(t, command)
	assert.Nil(t, err)
	assert.Equal(t, true, cc.IsRunning("test"))

	cont <- nil
	command.Wait()
	assert.Equal(t, false, cc.IsRunning("test"))

}
