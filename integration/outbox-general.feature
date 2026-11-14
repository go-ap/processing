Feature: Client to server interactions: Actor object's outbox
  Scenario: Accepts Activity Objects (outbox:accepts-activities) MUST
    Given an Activity Object
    Then it gets accepted

  Scenario: Accepts non-Activity Objects, and converts to Create Activities (outbox:accepts-non-activity-objects) MUST
    Given a non-Activity Object
    Then it Fails to get converted to Create Activity

  # GoActivityPub encourages downstream users to strip Bto and BCC in the handlers for **remote fetch**
  # We store the objects with them because all recipients form the object's ACL
  Scenario: Removes the bto and bcc properties from Objects before storage and delivery (outbox:removes-bto-and-bcc) MUST
    Given Objects with 'bto' and 'bcc' properties
    Then Skip
    But the 'bto' and 'bcc' aren't removed before storage and delivery

  Scenario: Ignores id on submitted activities, and generates a new id instead (outbox:ignores-id) MUST
    Given an Activity with existing ID
    Then ignores it and generates a new ID instead

  # Another job for the handler, not the processor itself.
  Scenario: Responds with status code 201 Created (outbox:responds-201-created) MUST
    Given an Activity Object
    Then Skip

  Scenario: Response includes Location header whose value is id of new object, unless the Activity is transient (outbox:location-header)
    Given an Activity Object
    Then Skip

  # This is not something that I've seen implemented in the wild.
  # The SWICG(Social Web Incubator Community Group) is still drafting a proposal for media upload
  Scenario: Accepts Uploaded Media in submissions (outbox:upload-media) MUST
    Given nothing
    Then Skip

  Scenario: Server does not trust client submitted content (outbox:not-trust-submitted) SHOULD
    Given an Activity Object
    But no Author
    Then there is a validation error

  # In GoActivityPub content validation is done at the presentation layer
  Scenario: Validate the content they receive to avoid content spoofing attacks (outbox:validate-content) SHOULD
    Given an Activity Object
    Then Skip

  # This is not really quantifiable. The client implementation that does the recipients dissemination can
  # have a staggering mechanism, but it's not in the scope of the processing module.
  Scenario: Take care not to overload other servers with delivery submissions (outbox:do-not-overload) SHOULD
    Given an Activity Object
    Then Skip
