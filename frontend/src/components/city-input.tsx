import { useId, useState, type FormEvent } from 'react'
import { Plus } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  MAX_TRACKED_CITIES,
  type AddCityResult,
} from '@/hooks/use-tracked-cities'

interface CityInputProps {
  /** Adds the city, returning whether it was accepted and why not. */
  onAdd: (city: string) => AddCityResult
  disabled?: boolean
}

/**
 * Form for adding a city to the tracked list.
 *
 * The request fires on submit rather than on keystroke, so there is nothing to
 * debounce: debouncing throttles work done per keystroke, and this field does
 * no work while the user types. Filtering is client-side and therefore instant,
 * which is the other place debounce is usually added for no benefit.
 */
export default function CityInput({ onAdd, disabled = false }: CityInputProps) {
  const [value, setValue] = useState('')
  const [feedback, setFeedback] = useState<string | null>(null)
  const inputId = useId()
  const feedbackId = `${inputId}-feedback`

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    const result = onAdd(value)

    if (result.ok) {
      setValue('')
      setFeedback(null)
      return
    }

    setFeedback(
      result.reason === 'duplicate'
        ? `${value.trim()} is already being tracked.`
        : result.reason === 'limit'
          ? `You can track up to ${MAX_TRACKED_CITIES} cities. Remove one to add another.`
          : 'Enter a city name to track.',
    )
  }

  return (
    <form onSubmit={handleSubmit} noValidate>
      <label
        htmlFor={inputId}
        className="text-sm font-medium text-muted-foreground"
      >
        Add a city
      </label>

      <div className="mt-2 flex flex-wrap gap-2">
        <Input
          id={inputId}
          value={value}
          onChange={(event) => {
            setValue(event.target.value)
            if (feedback !== null) setFeedback(null)
          }}
          placeholder="Search for a city"
          autoComplete="off"
          spellCheck={false}
          disabled={disabled}
          maxLength={60}
          aria-describedby={feedback !== null ? feedbackId : undefined}
          aria-invalid={feedback !== null}
          className="max-w-xs"
        />

        <Button type="submit" disabled={disabled || value.trim() === ''}>
          <Plus className="size-4" aria-hidden="true" />
          Add
        </Button>
      </div>

      {/* The live region exists only when there is something to announce.
          Rendering role="alert" permanently would make a screen reader
          announce an empty alert on every page load. A spacer keeps the form
          from shifting without exposing a live region. */}
      {feedback !== null ? (
        <p
          id={feedbackId}
          role="alert"
          className="mt-2 min-h-5 text-sm text-destructive"
        >
          {feedback}
        </p>
      ) : (
        <div className="mt-2 min-h-5" aria-hidden="true" />
      )}
    </form>
  )
}
