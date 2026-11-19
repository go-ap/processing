@conformance
Feature: Server to server interactions: Relationship management

  # Follow
  Scenario: Add the actor to the object user's Followers Collection (inbox:accept:follow:add-actor-to-users-followers) SHOULD
  Scenario: Generates either an Accept or Reject activity with Follow as object and deliver to actor of the Follow (inbox:accept:follow:generate-accept-or-reject) SHOULD
  Scenario: If in reply to a Follow activity, adds actor to receiver's Following Collection (inbox:accept:accept:add-actor-to-users-following) SHOULD
