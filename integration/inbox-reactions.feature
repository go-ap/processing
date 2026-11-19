@conformance
Feature: Server to server interactions: Reactions

  # Like
  Scenario: Perform appropriate indication of the like being performed (See 7.10 for examples) (inbox:accept:like:indicate-like-performed) SHOULD
  Scenario: Increments object's count of shares by adding the received activity to the 'shares' collection if this collection is present (inbox:accept:announce:add-to-shares-collection) SHOULD
