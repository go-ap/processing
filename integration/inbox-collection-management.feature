@conformance
Feature: Server to server interactions: Collection Management

  # Add
  Scenario: Add the object to the Collection specified in the target property, unless not allowed to per requirements in 7.8 (inbox:accept:add:to-collection) SHOULD
    When an Add activity is received
    Then the object will be added to the Target collection

  # Remove
  Scenario: Remove the object from the Collection specified in the origin property, unless not allowed per requirements in 7.9 (inbox:accept:remove:from-collection) SHOULD
    When a Remove activity is received
    Then the object will be removed from the Origin collection
