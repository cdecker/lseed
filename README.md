# lseed -- A Lightning DNS Seed

Upon first joining the Lightning Network, a node must open a few connections to existing nodes in the network.
However, it can only learn of nodes that are present in the network through its peers.
This Seed helps bootstrapping new nodes by indexing nodes that are present in the network and returning a random sample when queried.

In addition the seed provides a way to query for specific nodes, e.g., allowing nodes to quickly find their peers even though the switched IP or port.

## Supported Queries

Generally this implementation supports both IPv4 and IPv6 queries, i.e., `A` and `AAAA` queries.
In addition it supports `SRV` queries that return a mix of IPv4 nodes and IPv6 nodes, and their associated `A` and `AAAA` answers.

### A & AAAA Queries

The seed answers incoming `A` and `AAAA` queries with up to 25 known nodes in the network.
The nodes are filtered by their listening port, and only nodes that listen on the default Lightning port, 9735, are returned.
This is necessary since it is not possible to specify the port in `A` and `AAAA` answers.

### SRV Queries

Upon receiving an `SRV` query the seed will sample up to 25 nodes from the known nodes, regardless of their listening port, and return them, specifying an alias and the port.
The `SRV` query attempts to return a balanced set of IPv4 and IPv6 nodes.

In addition to the alias and port, the seed will also attach the matching `A` and `AAAA` records, such that a single query return both IP and port, and nodes may initiate connections without further queries.
 
## Node Queries (A & AAAA)

Given the alias from the `SRV` queries, a client can also directly query for a specific node.
If the node's ID is `03edd9462482dbe1f1ea75db38a345c2b1d8a325c5c86f72aa9ea191a94be8b664` then the corresponding alias will be:

    3edd9462482dbe1f1ea75db38a345c2b1d8a325c5c86f72aa9ea191a94be8b6.64.lseed.bitcoinstats.com
	
The leading `0` character is removed from the pubkey, and the pubkey is split into two chunks. The first chunk is 63 characters long, while the second chunk is 2 characters long.
The two chunks are then dot-separated and prefixed to the seed's domain, `lseed.bitcoinstats.com` in this case.

The answer contains the record matching the query, or the record of the other IP version type in the additional section if IP versions do not match. 

## Information Source

Currently the seed will poll a local Lightning node periodically and update its local view accordingly.
In future I'd like to introduce a number of different information sources and add further tests, such as testing for reachability before returning nodes.
