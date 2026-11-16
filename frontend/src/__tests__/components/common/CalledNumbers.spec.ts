import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import CalledNumbers from '@/components/common/CalledNumbers.vue'
import { axeFindings } from '@/__tests__/axe'

const noneCalled = () => false

describe('CalledNumbers', () => {
  it('renders five columns headed B-I-N-G-O', () => {
    const w = mount(CalledNumbers, { props: { count: 0, isCalled: noneCalled } })
    expect(w.findAll('.called-numbers__col-header').map((h) => h.text())).toEqual([
      'B',
      'I',
      'N',
      'G',
      'O',
    ])
  })

  it('dims only the columns marked inactive by activeColumns', () => {
    const w = mount(CalledNumbers, {
      props: { count: 0, isCalled: noneCalled, activeColumns: [true, false, false, false, true] },
    })
    const dimmed = w
      .findAll('.called-numbers__col')
      .map((c) => c.classes().includes('called-numbers__col--unused'))
    expect(dimmed).toEqual([false, true, true, true, false])
  })

  it('dims nothing when activeColumns is omitted', () => {
    const w = mount(CalledNumbers, { props: { count: 0, isCalled: noneCalled } })
    expect(w.findAll('.called-numbers__col.called-numbers__col--unused')).toHaveLength(0)
  })

  it('highlights a called number', () => {
    const w = mount(CalledNumbers, { props: { count: 1, isCalled: (n: number) => n === 5 } })
    const called = w.findAll('.called-numbers__num-cell.is-called')
    expect(called).toHaveLength(1)
    expect(called[0]!.text()).toBe('5')
  })

  it('has nothing axe flags', async () => {
    const w = mount(CalledNumbers, {
      props: {
        count: 1,
        isCalled: (n: number) => n === 5,
        activeColumns: [true, false, false, false, true],
      },
    })
    expect(await axeFindings(w.element)).toEqual([])
  })
})
