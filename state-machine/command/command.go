package command

import (
	"encoding/json"
	"fmt"
	"statemachine/constants"
	"statemachine/types"
)

type Command struct {
	Operation types.Operation
	Key       string
	Value     string
}

func EncodeCommand(cmd Command) ([]byte, error) {
	return json.Marshal(cmd)
}

func DecodeCommand(data []byte) (Command, error) {
	var cmd Command

	if err := json.Unmarshal(data, &cmd); err != nil {
		return Command{}, fmt.Errorf("Error decoding command: %w", err)
	}

	return cmd, nil
}

func (c Command) Validate() error {
	switch c.Operation {
	case constants.PUT_OPERATION, constants.DELETE_OPERATION:
	default:
		return fmt.Errorf("Unsupported operation type: %q", c.Operation)
	}

	if c.Key == "" {
		return fmt.Errorf("Command key cannot be empty")
	}

	return nil
}
