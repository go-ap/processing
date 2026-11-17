@conformance
Feature: Client to server interactions: Content Management

  # Create
  Scenario: Merges audience properties ('to', 'bto', 'cc', 'bcc', 'audience') with the Create's object's audience properties (outbox:create:merges-audience-properties) SHOULD
    When a Create and Object pair with to, bto, cc, bcc and audience is received
    Then Object and Activity have merged to, bto, cc, bcc and audience

  Scenario: Create's actor property is copied to be the value of .object.attributedTo (outbox:create:actor-to-attributed-to) SHOULD
    When a Create for an Object without AttributedTo is received
    Then the Object has the AttributedTo as the Activity Actor

  # Update
  Scenario: Server takes care to be sure that the Update is authorized to modify its object before modifying the server's stored copy (outbox:update:check-authorized) MUST
    When an Update with an unauthorized Actor is received
    Then there is a validation error

  Scenario: Supports partial updates in client-to-server protocol (but not server-to-server) (outbox:update:partial) NON-NORMATIVE
    When an Update with an Object containing only partial data is received
    Then no properties of the Object are unset
