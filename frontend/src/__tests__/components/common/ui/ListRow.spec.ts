import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ListRow from '@/components/common/ui/ListRow.vue'
import { axeFindings } from '@/__tests__/axe'

describe('ListRow', () => {
  it('renders the body (default slot)', () => {
    const wrapper = mount(ListRow, { slots: { default: '<h4>Title</h4>' } })
    expect(wrapper.find('.list-row__body h4').text()).toBe('Title')
  })

  it('renders media and actions slots, with actions pinned in their own region', () => {
    const wrapper = mount(ListRow, {
      slots: {
        media: '<img class="cover" />',
        default: 'body',
        actions: '<button class="del">x</button>',
      },
    })
    expect(wrapper.find('.list-row__media .cover').exists()).toBe(true)
    expect(wrapper.find('.list-row__actions .del').exists()).toBe(true)
  })

  it('omits media/actions wrappers when those slots are absent', () => {
    const wrapper = mount(ListRow, { slots: { default: 'body' } })
    expect(wrapper.find('.list-row__media').exists()).toBe(false)
    expect(wrapper.find('.list-row__actions').exists()).toBe(false)
  })

  it('has nothing axe flags', async () => {
    const wrapper = mount(ListRow, {
      slots: {
        media: '<img class="cover" src="cover.png" alt="Cover" />',
        default: '<h4>Title</h4>',
        actions: '<button type="button">Delete</button>',
      },
    })
    expect(await axeFindings(wrapper.element)).toEqual([])
  })
})
