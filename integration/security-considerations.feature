@conformance
Feature: Server to server interactions: Security Considerations

  Scenario: Server verifies that the new content is really posted by the actor indicated in Objects received in inbox and outbox (server:security-considerations:actually-posted-by-actor) NON-NORMATIVE
  Scenario: By default, implementation does not make HTTP requests to localhost when delivering Activities (server:security-considerations:do-not-post-to-localhost) NON-NORMATIVE
  Scenario: Implementation applies a whitelist of allowed URI protocols before issuing requests, e.g. for inbox delivery (server:security-considerations:uri-scheme-whitelist) NON-NORMATIVE
  Scenario: Server filters incoming content both by local untrusted users and any remote users through some sort of spam filter (server:security-considerations:filter-incoming-content) NON-NORMATIVE
