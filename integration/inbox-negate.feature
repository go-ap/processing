@conformance
Feature: Server to server interactions: Negate

  # Undo
  Scenario: Performs Undo of object in federated context (inbox:accept:undo) NON-NORMATIVE
    When an Undo activity is received
    Then processing is successful
