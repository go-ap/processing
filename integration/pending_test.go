package integration

import (
	"fmt"

	"github.com/cucumber/godog"
)

/*
 Feature
  Rule (as of Gherkin 6)
  Example (or Scenario)
  Given, When, Then, And, But for steps (or *)
  Background
  Scenario Outline (or Scenario Template)
  Examples (or Scenarios)

 https://cucumber.io/docs/gherkin/reference
*/

func fail() error {
	return fmt.Errorf("force fail")
}

func nothing() error {
	return nil
}

func skip() error {
	return godog.ErrSkip
}
