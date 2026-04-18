package forkcap

import "strings"

type Suite struct {
	Fork         string
	Capabilities []Capability
}

func DefaultSuite(fork string) (Suite, error) {
	fork = strings.TrimSpace(strings.ToLower(fork))
	if fork == "" {
		fork = "all"
	}
	if err := ValidateForkSelection(fork); err != nil {
		return Suite{}, err
	}
	return Suite{
		Fork:         fork,
		Capabilities: FilterByFork(DefaultRegistry(), fork),
	}, nil
}
