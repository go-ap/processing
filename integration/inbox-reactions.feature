@conformance
Feature: Server to server interactions: Reactions

  # Like
  Scenario: Perform appropriate indication of the like being performed (See 7.10 for examples) (inbox:accept:like:indicate-like-performed) SHOULD
    When receiving a Like activity
    Then the processing is successful and the likes collection contains the activity
