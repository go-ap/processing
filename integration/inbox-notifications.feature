@conformance
Feature: Server to server interactions: Notifications

  # Announce
  Scenario: Increments object's count of shares by adding the received activity to the 'shares' collection if this collection is present (inbox:accept:announce:add-to-shares-collection) SHOULD
    When receiving an Announce activity
    Then the processing is successful and the shares collection contains the activity
