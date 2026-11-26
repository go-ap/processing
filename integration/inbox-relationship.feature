@conformance
Feature: Server to server interactions: Relationship management

  # Currently we support the flow of directly adding a follower only for non Person actors
  Scenario: Add the actor to the object user's Followers Collection (inbox:accept:follow:add-actor-to-users-followers) SHOULD
    When receiving a Follow activity in a Group actor's inbox
    Then it gets accepted
    And we add the Follow's activity actor to the receiver's following collection

  Scenario: Generates either an Accept or Reject activity with Follow as object and deliver to actor of the Follow (inbox:accept:follow:generate-accept-or-reject) SHOULD
    When receiving a Follow activity
    Then it gets accepted
    And we respond with an Accept activity

  Scenario: If in reply to a Follow activity, adds actor to receiver's Following Collection (inbox:accept:accept:add-actor-to-users-following) SHOULD
    When receiving a Follow activity
    And we send an Accept activity in response
    Then we add the Follow's activity actor to the receiver's following collection
