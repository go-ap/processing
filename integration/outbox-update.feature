Feature: Client to server interactions: Update

  Scenario: Server takes care to be sure that the Update is authorized to modify its object before modifying the server's stored copy (outbox:update:check-authorized) MUST
    Given an Update with an unauthorized Actor
    Then there is a validation error

  Scenario: Supports partial updates in client-to-server protocol (but not server-to-server) (outbox:update:partial) NON-NORMATIVE
    Given an Update with an Object containing only partial data
    Then no properties of the Object are unset