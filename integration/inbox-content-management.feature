@conformance
Feature: Server to server interactions: Content Management

  # Create
  Scenario: Supports receiving a Create object in an actor's inbox (inbox:accept:create) NON-NORMATIVE
    When a Create activity is received
    Then processing is successful and object matches

  # Update
  Scenario: Take care to be sure that the Update is authorized to modify its object (inbox:accept:update:is-authorized) MUST
    When an Update activity with an actor that is not the author of the updated object is received
    Then there is a validation error

  Scenario: Completely replace its copy of the activity with the newly received value (inbox:accept:update:completely-replace) SHOULD
    When an Update activity is received
    Then the object is fully replaced by the activity's object

  # Currently there are no valid authorization mechanisms for allowing edit access to objects outside of validating
  # the object's author.
  Scenario: Don't trust content received from a server other than the content's origin without some form of verification (inbox:accept:dont-blindly-trust) SHOULD
    When an Update activity is received from a server authority other than the original's object
    Then there is a validation error

  # Delete
  Scenario: Assuming object is owned by sending actor/server, removes object's representation (inbox:accept:delete) SHOULD
    When a Delete activity is received
    Then processing is successful

  Scenario: Replace object's representation with a Tombstone object (inbox:accept:delete:tombstone) MAY
    When a Delete activity is received
    Then the object is replaced with a Tombstone
