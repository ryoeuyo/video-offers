import { AuthForm } from '../components/AuthForm'

export function LoginPage() {
  return <AuthForm mode="login" />
}

export function RegisterPage() {
  return <AuthForm mode="register" />
}
