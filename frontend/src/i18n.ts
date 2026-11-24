import { ref } from 'vue'

export type Locale = 'en' | 'id'

const dictionary: Record<Locale, Record<string, string>> = {
  en: {
    'nav.dashboard': 'Dashboard',
    'nav.playground': 'Playground',
    'nav.settings': 'Settings',
    'auth.signIn': 'Sign in',
    'auth.register': 'Create admin account',
    'auth.guest': 'Continue as guest',
    'auth.email': 'Email address',
    'auth.password': 'Password',
    'dashboard.title': 'Projects',
    'dashboard.new': 'New project',
    'playground.send': 'Send',
    'playground.stop': 'Stop generation',
    'settings.title': 'Settings',
    'settings.health': 'Setup & health',
    'settings.users': 'Users',
    'common.save': 'Save',
    'common.cancel': 'Cancel'
  },
  id: {
    'nav.dashboard': 'Dasbor',
    'nav.playground': 'Playground',
    'nav.settings': 'Pengaturan',
    'auth.signIn': 'Masuk',
    'auth.register': 'Buat akun admin',
    'auth.guest': 'Lanjut sebagai tamu',
    'auth.email': 'Alamat email',
    'auth.password': 'Kata sandi',
    'dashboard.title': 'Project',
    'dashboard.new': 'Project baru',
    'playground.send': 'Kirim',
    'playground.stop': 'Hentikan jawaban',
    'settings.title': 'Pengaturan',
    'settings.health': 'Pemeriksaan setup',
    'settings.users': 'Pengguna',
    'common.save': 'Simpan',
    'common.cancel': 'Batal'
  }
}

const stored = typeof localStorage !== 'undefined' ? localStorage.getItem('rag_studio_locale') : null
export const locale = ref<Locale>(stored === 'id' ? 'id' : 'en')

export function setLocale(value: Locale) {
  locale.value = value
  if (typeof localStorage !== 'undefined') localStorage.setItem('rag_studio_locale', value)
}

export function t(key: string): string {
  return dictionary[locale.value][key] || dictionary.en[key] || key
}
