export interface DomainSite {
    id?: string;
    domain: string;
}

const domainCollator = new Intl.Collator('en', {
    numeric: true,
    sensitivity: 'base'
});

export function compareSitesByDomain(left: DomainSite, right: DomainSite): number {
    const domainCompare = domainCollator.compare(left.domain, right.domain);
    if (domainCompare !== 0) {
        return domainCompare;
    }
    return domainCollator.compare(left.id ?? '', right.id ?? '');
}

export function sortSitesByDomain<TSite extends DomainSite>(sites: readonly TSite[]): TSite[] {
    return [...sites].sort(compareSitesByDomain);
}
