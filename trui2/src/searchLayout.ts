// Раскладки клавиатуры: позиция символа в одной строке соответствует
// символу на той же физической клавише в другой строке.
const ru = 'йцукенгшщзхъфывапролджэячсмитьбю.ЙЦУКЕНГШЩЗХЪФЫВАПРОЛДЖЭЯЧСМИТЬБЮ,'
const en = 'qwertyuiop[]asdfghjkl;\'zxcvbnm,./QWERTYUIOP{}ASDFGHJKL:"ZXCVBNM<>?'

/**
 * Переводит текст в другую раскладку: если символ английский — в русский,
 * иначе наоборот. Символы, которых нет ни в одной раскладке, остаются как есть.
 */
export function switchLayout(text: string): string {
  let result = ''
  for (let i = 0; i < text.length; i++) {
    const char = text[i]
    let pos = en.indexOf(char)
    if (pos !== -1) {
      result += ru[pos]
    } else {
      pos = ru.indexOf(char)
      result += pos !== -1 ? en[pos] : char
    }
  }
  return result
}

/**
 * Варианты поискового запроса с учётом возможной неверной раскладки.
 * Возвращает нормализованные строки (lowercase, trim) без дублей.
 * Пустой ввод → пустой массив (фильтр ничего не отсеивает).
 */
export function searchVariants(input: string): string[] {
  const base = input.trim().toLowerCase()
  if (!base) return []
  const switched = switchLayout(base)
  return switched === base ? [base] : [base, switched]
}
