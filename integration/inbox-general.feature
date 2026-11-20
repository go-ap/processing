@conformance
Feature: Server to server interactions: Actor's Inbox

  # In GoActivityPub content validation is done at the presentation layer
  Scenario: Validate the content they receive to avoid content spoofing attacks (inbox:accept:validate-content) SHOULD
    When an Activity is received
    Then the activity is disseminated to local recipients

  # I'm not sure what this is supposed to do
  #Scenario: Deduplicates activities returned by the inbox by comparing activity ids (inbox:accept:deduplicate) MUST
  Scenario: Forwards incoming activities to the values of to, bto, cc, bcc, audience if and only if criteria in 7.1.2 are met (inbox:accept:special-forward) MUST
    When an Activity with to, bto, cc and bcc recipients is received
    Then the activity is disseminated to local recipients

  Scenario: Recurse through to, bto, cc, bcc, audience object values to determine whether/where to forward according to criteria in 7.1.2 (inbox:accept:special-forward:recurses) SHOULD
    When an Activity with a local collection as a recipient is received
    Then the activity is forwarded to the remote actors part of the collection

  #Scenario: Limit recursion in this process (inbox:accept:special-forward:limits-recursion) SHOULD
