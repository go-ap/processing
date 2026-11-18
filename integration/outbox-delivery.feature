@conformance
Feature: Client to server interactions: Activities Delivery

  Scenario: Performs delivery on all Activities posted to the outbox (outbox:delivery:performs-delivery) MUST
    When an Activity is received
    Then it gets delivered to remote recipients

  Scenario: Utilizes to, bto, cc, and bcc to determine delivery recipients. (outbox:delivery:addressing) MUST
    When an Activity is received with to, bto, cc and bcc recipients is received
    Then it gets delivered to all recipients

  Scenario: Provides an id all Activities sent to other servers, unless the activity is intentionally transient (outbox:delivery:adds-id) MUST
    When an Activity without an ID is received
    Then it gets delivered with an ID

  Scenario: Dereferences delivery targets with the submitting user's credentials (outbox:delivery:submit-with-credentials) MUST
    When an Activity with non-public remote recipient is received
    Then the recipient is dereferenced using the author's credentials

  Scenario: Delivers to all items in recipients that are Collections or OrderedCollections (outbox:delivery:deliver-to-collection) MUST
    When an Activity with a recipient being a local collection is received
    Then all the collection members receive the Activity

  Scenario: Delivers activity with object property if the Activity type is one of Create, Update, Delete, Follow, Add, Remove, Like, Block, Undo (outbox:delivery:delivers-with-object-for-certain-activities) MUST
    When a Create Activity is received
    Then the recipients receive a non flattened Activity Object

  Scenario: Delivers activity with 'target' property if the Activity type is one of Add, Remove (outbox:delivery:delivers-with-target-for-certain-activities) MUST
    When an Add Activity is received
    Then the recipients receive a non flattened Target

  Scenario: Deduplicates final recipient list (outbox:delivery:deduplicates-final-recipient-list) MUST
    When an Activity with duplicate recipients is received
    Then the recipients are deduplicated

  Scenario: Does not deliver to recipients which are the same as the actor of the Activity being notified about (outbox:delivery:do-not-deliver-to-actor) MUST
    When an Activity with its Actor as a recipient is received
    Then the Actor doesn't get a copy in their inbox

  Scenario: Does not deliver Block Activities to their object (outbox:delivery:do-not-deliver-block) SHOULD NOT
    When a Block Activity is received
    Then the Blocked object gets removed from the recipients list

  Scenario: Delivers to sharedInbox endpoints to reduce the number of receiving actors delivered to by identifying all followers which share the same sharedInbox who would otherwise be individual recipients and instead deliver objects to said sharedInbox (outbox:delivery:sharedInbox) MAY
    When an Activity with recipients that have a sharedInbox is received
    Then the Activity is disseminated to the sharedInbox, not to the recipients inboxes

  Scenario: Deliver to actor inboxes and collections otherwise addressed which do not have a sharedInbox (outbox:delivery:sharedInbox:deliver-to-inbox-if-no-sharedInbox) MUST
    When an Activity with recipients is received
    Then the Activity is disseminated to the recipients that don't have sharedInboxes
