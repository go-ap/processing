@conformance
Feature: Client to server interactions: Reactions

  # GoActivityPub does not immediately add an actor to a follower's collection.
  # An Accept from the actor being followed is needed.
  Scenario: Adds followed object to the actor's Following Collection (outbox:follow:adds-followed-object) SHOULD
    When a Follow activity between two local Actors
    But the Actor is *not* added to the Followers collection

  Scenario: Adds the object to the actor's Liked Collection (outbox:like:adds-object-to-liked) SHOULD
    When receiving a Like activity
    Then the object is added to the actor's Liked collection
    And the activity is added to the object's Likes collection
