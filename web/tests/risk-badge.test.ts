import { mount } from '@vue/test-utils'
import RiskBadge from '../src/components/RiskBadge.vue'

describe('RiskBadge', () => {
  it('renders degraded state explicitly instead of normal', () => {
    const wrapper = mount(RiskBadge, { props: { level: 'degraded' } })
    expect(wrapper.text()).toContain('降级')
    expect(wrapper.classes()).toContain('risk-degraded')
  })
})
