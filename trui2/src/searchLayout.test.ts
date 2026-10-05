import { searchVariants, switchLayout } from './searchLayout'

describe('switchLayout', () => {
  it('переводит английский в русский', () => {
    expect(switchLayout('sber')).toBe('ыиук')
  })

  it('переводит русский в английский', () => {
    expect(switchLayout('ыиук')).toBe('sber')
  })

  it('оставляет символы вне раскладок как есть', () => {
    expect(switchLayout('sber-01!')).toBe('ыиук-01!')
  })
})

describe('searchVariants', () => {
  it('пустой ввод даёт пустой список', () => {
    expect(searchVariants('   ')).toEqual([])
  })

  it('даёт оба варианта раскладки в нижнем регистре', () => {
    expect(searchVariants('SBER').sort()).toEqual(['sber', 'ыиук'])
  })

  it('не дублирует вариант, если раскладка не меняет текст', () => {
    expect(searchVariants('123')).toEqual(['123'])
  })
})
