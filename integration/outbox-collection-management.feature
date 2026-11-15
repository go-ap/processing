@conformance
Feature: Client to server interactions: Collection Management

  Scenario: Adds object to the target Collection, unless not allowed due to requirements in 7.5 (outbox:add:adds-object-to-target) SHOULD
    Given an Object and a Target collection
    Then the object will exist in the Target

  Scenario: Remove object from the origin Collection, unless not allowed due to requirements in 7.5 (outbox:remove:removes-from-target) SHOULD
    Given an Object and an Origin collection
    Then the object will not exist in the Origin