import { Component } from '@angular/core';

@Component({
  selector: 'llm-skeleton-block',
  standalone: true,
  template: `
    <div class="flex flex-col gap-6 mb-6 w-full opacity-60">
      <!-- Simulated Model Message Skeleton -->
      <div class="w-full p-5 bg-white border border-gray-100 rounded-xl shadow-sm animate-pulse flex flex-col gap-4">
        <div class="flex items-center gap-3">
          <div class="w-6 h-6 rounded bg-gray-200"></div>
          <div class="h-3 bg-gray-200 rounded w-24"></div>
        </div>
        <div class="flex flex-col gap-2 pl-9">
          <div class="h-3 bg-gray-200 rounded w-full"></div>
          <div class="h-3 bg-gray-200 rounded w-11/12"></div>
          <div class="h-3 bg-gray-200 rounded w-4/5"></div>
        </div>
      </div>

      <!-- Simulated User Prompt Skeleton -->
      <div class="w-full p-5 bg-gray-50 border border-gray-200 rounded-xl shadow-sm animate-pulse flex flex-col gap-4">
        <div class="flex items-center gap-3">
          <div class="w-6 h-6 rounded-full bg-gray-300"></div>
          <div class="h-3 bg-gray-300 rounded w-16"></div>
        </div>
        <div class="flex flex-col gap-2 pl-9">
          <div class="h-3 bg-gray-300 rounded w-3/4"></div>
          <div class="h-3 bg-gray-300 rounded w-1/2"></div>
        </div>
      </div>
    </div>
  `
})
export class SkeletonBlockComponent {}