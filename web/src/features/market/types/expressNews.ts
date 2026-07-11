export type ExpressNewsTag = '' | 'A股' | '港股' | '美股' | '异动'

export interface ExpressNewsEntity {
  code: string
  name: string
  market: string
  exchange?: string
  price?: string
  ratio?: string
  changePct?: number
  logoUrl?: string
}

export interface ExpressNewsItem {
  id: string
  title: string
  body: string
  publishTime: number
  provider: string
  tag?: string
  important?: boolean
  thirdUrl?: string
  entities?: ExpressNewsEntity[]
}

export interface ExpressNewsResponse {
  tag: string
  pn: number
  rn: number
  source: string
  fetchedAt: number
  hasMore: boolean
  items: ExpressNewsItem[]
}

export const EXPRESS_NEWS_TABS: { label: string; value: ExpressNewsTag }[] = [
  { label: '全部', value: '' },
  { label: 'A股', value: 'A股' },
  { label: '港股', value: '港股' },
  { label: '美股', value: '美股' },
  { label: '异动', value: '异动' },
]
