# Domain glossary

| Term | Meaning in this client |
| --- | --- |
| Device | An inventory record identified by its canonical IP. Interface addresses are Device IP records. |
| Port | A physical or logical interface identified by device IP and the server's port name. Port names may contain slashes. |
| Node | A learned endpoint MAC and its sightings, not another name for a managed device. |
| Sighting | A node observed on a device port at recorded times; active and historical sightings differ. |
| VLAN | A numeric broadcast-domain identifier. VLAN membership, native PVID and textual port VLAN labels remain separate. |
| Tenant | A selected inventory database. Authentication and token issuance use the default authentication database. |
| Report | A server-enabled tabular query, identified by category and tag. Custom report columns belong to the installation. |
| Job | A queue record for backend work. Submission and completion are different events. |
| Skiplist | Backend deferral records for devices/actions. Queue deletion can remove these alongside jobs. |
| Import | A node/MAC or ARP/neighbor batch submitted to a worker route. HTTP success does not prove processing. |
| Token | An API credential issued or renewed by the authentication database. Permanent status is a server grant. |
| Token ACL | A source-address Host Group ACL on a token, not a route permission list. |
| Session | Cookie-backed authentication state used by logout and tenant forwarding. It is separate from a token. |
| Neighbor map | Upstream topology nodes and links. Filtering may retain links whose endpoint nodes are absent. |
| Projection | Explicit device columns selected for search. An unselected nullable field remains nil. |
