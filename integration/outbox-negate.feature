@conformance
Feature: Client to server interactions: Negating activities
  Scenario: Supports the Undo activity in the client-to-server protocol (outbox:undo) NON-NORMATIVE
    Given an Undo activity
    Then it gets accepted

  Scenario: Ensures that the actor in the activity actor is the same in activity being undone (outbox:undo:ensures-activity-and-actor-are-same) MUST
    Given an Undo activity with different Actor than the one of it's Object
    Then there is a validation error
