export interface TraceStep {
  type: 'PRE-VERIFY' | 'VERIFY' | 'SYSTEM';
  command: string;
  status: 'SUCCESS' | 'FAILED' | 'TIMEOUT' | 'UNKNOWN';
  output: string;
  isBestEffort: boolean;
}

export function hasStructuredTrace(message: string): boolean {
  if (!message) return false;
  return /\[(PRE-VERIFY|VERIFY)\].*?:/.test(message);
}

export function extractAttemptPrefix(message: string): string {
  const match = message.match(/^(\[Attempt \d+\] Error:\s*)/);
  return match ? match[1] : '';
}

export function parseVerificationTrace(rawTrace: string): TraceStep[] {
  if (!rawTrace) return [];
  
  const steps: TraceStep[] = [];
  let currentStep: TraceStep | null = null;
  
  let traceToParse = rawTrace;
  const prefix = extractAttemptPrefix(rawTrace);
  if (prefix) {
    traceToParse = rawTrace.substring(prefix.length);
  }

  const lines = traceToParse.split('\n');
  for (const line of lines) {
    const match = line.match(/^\[(PRE-VERIFY|VERIFY)\]\s+(.*?):\s+(SUCCESS|FAILED|TIMEOUT)/);
    
    if (match) {
      if (currentStep) {
        currentStep.output = currentStep.output.trim();
        steps.push(currentStep);
      }
      currentStep = {
        type: match[1] as 'PRE-VERIFY' | 'VERIFY',
        command: match[2].trim(),
        status: match[3] as 'SUCCESS' | 'FAILED' | 'TIMEOUT' | 'UNKNOWN',
        output: '',
        isBestEffort: match[1] === 'PRE-VERIFY'
      };
    } else {
      if (currentStep) {
        currentStep.output += line + '\n';
      } else if (line.trim()) {
        // Fallback for raw, unformatted system panics or patcher errors
        currentStep = {
          type: 'SYSTEM',
          command: prefix.trim() || 'System Error',
          status: 'UNKNOWN',
          output: line + '\n',
          isBestEffort: false
        };
      }
    }
  }
  
  if (currentStep) {
    currentStep.output = currentStep.output.trim();
    steps.push(currentStep);
  }
  
  return steps;
}