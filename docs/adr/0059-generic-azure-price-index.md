# ADR-0059: A rough price index for the other Azure services

Status: accepted

## Context
Region-move recommendations show a cost effect only where a price index describes the service (virtual machines, managed databases and caches; ADR-0013 and later). For
App Service, Container Apps, Cognitive Search, Foundry models and the like the card said "cost: awaiting price data", although Azure's public retail price list has the data
(checked: Container Apps has 16 meters in both Canadian regions and all 16 cost the same). The gap was the lack of an index, not of data.

## Decision
`platform/azureprice` builds, for a short list of services (`GenericServices`: App Service, Container Apps, Functions, Cognitive Search, Cosmos DB, Foundry Models, Kubernetes Service, Monitor),
a **generic family**: no curated basket; every consumption meter the service has in the baseline region and in the target region is compared and the index is the geometric mean of the price
ratios, with at least five meters in common (otherwise no index for that region). Spot and low-priority meters are skipped. The daily price job builds them like the other families
(ids `azure:svc:<slug>`); the engine finds a service's index by the same slug (a test keeps the two in step).

It is a **rough** figure and is shown as one: a recommendation priced from such an index has `cost_basis` `region_price_list`, its confidence is multiplied by 0.8, the cards say "(rough)"
with a tooltip that it is the ratio of list prices over every meter, not of a basket, and that it does not know the customer's usage mix. **A rough estimate is never added into a
headline figure** (the Overview's potential saving and "What we found" leave `region_price_list` out and count it as not estimated). Services whose price does not depend on the region
(Azure DevOps has no regional price at all), storage and networking are not on the list and stay "not estimated". A zero effect is shown as "same price", not as a saving of zero.

## Consequences
On the stand: App Service Canada Central to East is about 4.4% cheaper (rough), Container Apps and Foundry about the same, Cognitive Search 0.6% more, DevOps still not estimated.
Eight more families add roughly 8 x 40 regions of price-list requests to the daily job. An unweighted mean can mislead for a service whose bill is dominated by one meter; that is why it
is labelled and kept out of the totals. A weighted index (by the customer's own meters, which the billing export names) is the next step if the rough one proves too coarse.
