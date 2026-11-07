# About GoActivityPub: Processing

[![Build Status](https://builds.sr.ht/~mariusor/processing.svg)](https://builds.sr.ht/~mariusor/processing)
[![Coveralls](https://img.shields.io/coverallsCoverage/github/go-ap/processing)](https://coveralls.io/github/go-ap/processing)
[![MIT Licensed](https://img.shields.io/github/license/go-ap/processing.svg)](https://raw.githubusercontent.com/go-ap/processing/master/LICENSE)

This project is part of the [GoActivityPub](https://github.com/go-ap) library which helps with creating ActivityPub applications using the Go programming language.

It provides basic functionality for processing and validation of generic ActivityPub activities.

It uses the concepts detailed in the [5.8.1 Content Management](https://www.w3.org/TR/activitystreams-vocabulary/#motivations)
section of the ActivityStreams vocabulary specification for logic separation between groups of activities.

For the actual processing it uses the [6. Client to Server Interactions](https://www.w3.org/TR/activitypub/#client-to-server-interactions)
and [7. Server to Server Interactions](https://www.w3.org/TR/activitypub/#server-to-server-interactions) sections of the ActivityPub specification.

## Extended documentation

For more information about how the module works in the scope of the
GoActivityPub library, see the [expanded
documentation](https://go-activitypub.federated.id/lib/modules/#processing).

For discussions about the projects you can write to the discussions mailing list: [~mariusor/go-activitypub-discuss@lists.sr.ht](mailto:~mariusor/go-activitypub-discuss@lists.sr.ht)

For patches and bug reports please use the dev mailing list: [~mariusor/go-activitypub-dev@lists.sr.ht](mailto:~mariusor/go-activitypub-dev@lists.sr.ht)
