import { useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import axios from 'axios'

interface SignupProps {
  setIsAuthenticated: (value: boolean) => void
}

export default function Signup({ setIsAuthenticated }: SignupProps) {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()

  const validatePassword = (pwd: string) => {
    if (pwd.length < 12) return 'Password must be at least 12 characters'
    if (!/[A-Z]/.test(pwd)) return 'Password must contain uppercase letter'
    if (!/[a-z]/.test(pwd)) return 'Password must contain lowercase letter'
    if (!/[0-9]/.test(pwd)) return 'Password must contain number'
    if (!/[!@#$%^&*]/.test(pwd)) return 'Password must contain special character (!@#$%^&*)'
    return ''
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')

    const pwdError = validatePassword(password)
    if (pwdError) {
      setError(pwdError)
      return
    }

    if (password !== confirmPassword) {
      setError('Passwords do not match')
      return
    }

    setLoading(true)

    try {
      const response = await axios.post('/api/v1/auth/signup', { email, password })
      localStorage.setItem('jwt_token', response.data.jwt_token)
      localStorage.setItem('user_id', response.data.user_id)
      setIsAuthenticated(true)
      navigate('/dashboard')
    } catch (err: any) {
      setError(err.response?.data?.error || 'Signup failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-primary-50 to-blue-50 flex items-center justify-center p-4">
      <div className="bg-white rounded-lg shadow-xl p-8 w-full max-w-md">
        <h1 className="text-3xl font-bold text-gray-900 mb-2">Create Account</h1>
        <p className="text-gray-600 mb-8">Monitor your APIs in 2 minutes</p>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Email</label>
            <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-600" placeholder="you@example.com" required />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Password</label>
            <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-600" placeholder="••••••••" required />
            <p className="text-xs text-gray-500 mt-1">Min 12 chars, uppercase, lowercase, number, special char (!@#$%^&*)</p>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Confirm Password</label>
            <input type="password" value={confirmPassword} onChange={(e) => setConfirmPassword(e.target.value)} className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary-600" placeholder="••••••••" required />
          </div>

          {error && <div className="p-3 bg-red-50 border border-red-200 text-red-700 rounded-lg text-sm">{error}</div>}

          <button type="submit" disabled={loading} className="w-full bg-primary-600 text-white py-2 rounded-lg font-medium hover:bg-primary-700 transition disabled:opacity-50">
            {loading ? 'Creating account...' : 'Sign Up'}
          </button>
        </form>

        <p className="text-center text-gray-600 mt-6">
          Already have an account? <Link to="/login" className="text-primary-600 hover:underline font-medium">Login</Link>
        </p>
      </div>
    </div>
  )
}
