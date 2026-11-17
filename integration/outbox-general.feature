@conformance
Feature: Client to server interactions: Actor object's outbox
  Scenario: Accepts Activity Objects (outbox:accepts-activities) MUST
    When an Activity is received
    Then it gets accepted

  Scenario: Accepts non-Activity Objects, and converts to Create Activities (outbox:accepts-non-activity-objects) MUST
    When a non-Activity Object is received
    Then Skip
    Then it gets wrapped into a Create Activity

  # GoActivityPub encourages downstream users to strip Bto and BCC in the handlers for **remote fetch**
  # We store the objects with them because all recipients form the object's ACL
  Scenario: Removes the `bto` and `bcc` properties from Objects before storage and delivery (outbox:removes-bto-and-bcc) MUST
    When an Activity with an Object having 'bto' and 'bcc' properties is received
    Then Skip
    But the 'bto' and 'bcc' aren't removed before storage and delivery

  Scenario: Ignores id on submitted activities, and generates a new id instead (outbox:ignores-id) MUST
    When an Activity with existing ID is received
    Then ignores it and generates a new ID instead

  # Another job for the handler, not the processor itself.
  Scenario: Responds with status code 201 Created (outbox:responds-201-created) MUST
    When an Activity is received
    Then Skip

  Scenario: Response includes Location header whose value is id of new object, unless the Activity is transient (outbox:location-header)
    When an Activity is received
    Then Skip

  # This is not something that I've seen implemented in the wild.
  # The SWICG(Social Web Incubator Community Group) is still drafting a proposal for media upload
  Scenario: Accepts Uploaded Media in submissions (outbox:upload-media) MUST
    Then Skip

  Scenario: Server does not trust client submitted content (outbox:not-trust-submitted) SHOULD
    When an Activity is received
    But no Author
    Then there is a validation error

  # In GoActivityPub content validation is done at the presentation layer
  Scenario: Validate the content they receive to avoid content spoofing attacks (outbox:validate-content) SHOULD
    When an Activity is received
    Then Skip

  # This is not really quantifiable. The client implementation that does the recipients dissemination can
  # have a staggering mechanism, but it's not in the scope of the processing module.
  Scenario: Take care not to overload other servers with delivery submissions (outbox:do-not-overload) SHOULD
    When an Activity is received
    Then Skip
