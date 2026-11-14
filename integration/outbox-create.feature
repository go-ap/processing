Feature: Client to server interactions: Create

  Scenario: Merges audience properties ('to', 'bto', 'cc', 'bcc', 'audience') with the Create's object's audience properties (outbox:create:merges-audience-properties) SHOULD
    Given a Create and Object pair with to, bto, cc, bcc and audience
    Then Object and Activity have merged to, bto, cc, bcc and audience

  Scenario: Create's actor property is copied to be the value of .object.attributedTo (outbox:create:actor-to-attributed-to) SHOULD
    Given a Create for an Object without AttributedTo
    Then the Object has the AttributedTo as the Activity Actor