<script lang="ts">
  // Arrow pointing in the direction of a shot from the centre, with a
  // circle for an inner ten (as on the former PDF report).
  import type { Shot, TargetGeometry } from './api'

  let { shot, geometry }: { shot: Shot; geometry: TargetGeometry } = $props()

  const innerTen = $derived(Math.hypot(shot.x, shot.y) - geometry.radiusCalibre / 100 < geometry.radiusInnerTen / 100)
  // SVG y points down; the arrow is drawn pointing right and rotated.
  const angle = $derived((-Math.atan2(shot.y, shot.x) * 180) / Math.PI)
</script>

<svg class="arrow" viewBox="-10 -10 20 20" aria-hidden="true">
  {#if innerTen}<circle r="8.5" />{/if}
  {#if shot.x !== 0 || shot.y !== 0}
    <g transform="rotate({angle})">
      <line x1="-8.5" y1="0" x2="8.5" y2="0" />
      <polyline points="3.5,-5 8.5,0 3.5,5" />
    </g>
  {/if}
</svg>

<style>
  .arrow {
    width: 1.1em;
    height: 1.1em;
    display: block;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.6;
    stroke-linecap: round;
    stroke-linejoin: round;
  }
</style>
