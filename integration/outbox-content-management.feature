@conformance
Feature: Client to server interactions: Content Management

  # Create
  Scenario: Merges audience properties ('to', 'bto', 'cc', 'bcc', 'audience') with the Create's object's audience properties (outbox:create:merges-audience-properties) SHOULD
    Given a Create and Object pair with to, bto, cc, bcc and audience
    Then Object and Activity have merged to, bto, cc, bcc and audience

  Scenario: Create's actor property is copied to be the value of .object.attributedTo (outbox:create:actor-to-attributed-to) SHOULD
    Given a Create for an Object without AttributedTo
    Then the Object has the AttributedTo as the Activity Actor

  # Update
  Scenario: Server takes care to be sure that the Update is authorized to modify its object before modifying the server's stored copy (outbox:update:check-authorized) MUST
    Given an Update with an unauthorized Actor
    Then there is a validation error

  Scenario: Supports partial updates in client-to-server protocol (but not server-to-server) (outbox:update:partial) NON-NORMATIVE
    Given an Update with an Object containing only partial data
    Then no properties of the Object are unset
