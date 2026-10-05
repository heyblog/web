import * as THREE from 'three';

import type { SiteGraph } from '../../api/sites/site-graph.types.ts';

import { createGraphControls } from './site-graph.controls.browser.ts';
import type { GraphInteraction } from './site-graph.input.browser.ts';
import type { GraphLabelNode } from './site-graph.labels.ts';
import { graphArrowMaterial, graphLineMaterial } from './site-graph.lines.browser.ts';
import { indexGraph } from './site-graph.model.ts';
import { graphLineOpacity, graphNodeSize, type GraphPoint } from './site-graph.navigation.ts';
import { createGraphPoints } from './site-graph.points.browser.ts';
import { graphArrows, graphPalette, pathEdges } from './site-graph.scene.browser.ts';
import { graphFrame } from './site-graph.view.ts';

export function createGraphRenderer(
  host: HTMLElement,
  graph: SiteGraph,
  callbacks: {
    readonly select: (id: string) => void;
    readonly focus: (id: string) => void;
    readonly zoom: (zoom: number) => void;
    readonly labels: (value: readonly GraphLabelNode[]) => void;
  },
) {
  const renderer = new THREE.WebGLRenderer({ antialias: true });
  const ratio = Math.min(window.devicePixelRatio, 1.5);
  renderer.setPixelRatio(ratio);
  const canvas = renderer.domElement;
  canvas.setAttribute('aria-label', '博客友链三维图谱，方向键平移，加减键缩放，Home 适配视图');
  canvas.setAttribute('role', 'group');
  canvas.tabIndex = 0;
  host.append(canvas);
  const scene = new THREE.Scene();
  const camera = new THREE.PerspectiveCamera(50, 1, 0.1, 100_000);
  const index = indexGraph(graph);
  const points = createGraphPoints(graph, ratio);
  scene.add(points.mesh);
  const edges = graph.edges.filter((edge) => !edge.reciprocal || edge.source < edge.target);
  const coordinates = new Float32Array(edges.length * 6);
  const colors = new Float32Array(coordinates.length);
  const strengths = new Float32Array(edges.length * 2);
  const lines = new THREE.BufferGeometry();
  lines.setAttribute('position', new THREE.BufferAttribute(coordinates, 3));
  lines.setAttribute('color', new THREE.BufferAttribute(colors, 3));
  lines.setAttribute('strength', new THREE.BufferAttribute(strengths, 1));
  const lineMaterial = graphLineMaterial();
  const lineMesh = new THREE.LineSegments(lines, lineMaterial);
  lineMesh.frustumCulled = false;
  scene.add(lineMesh);
  const arrows = new THREE.BufferGeometry();
  const arrowPositions = new Float32Array(graph.edges.length * 12);
  const arrowAnchors = new Float32Array(arrowPositions.length);
  const arrowPositionAttribute = new THREE.BufferAttribute(arrowPositions, 3);
  const arrowAnchorAttribute = new THREE.BufferAttribute(arrowAnchors, 3);
  arrows.setAttribute('position', arrowPositionAttribute);
  arrows.setAttribute('anchor', arrowAnchorAttribute);
  arrows.setDrawRange(0, 0);
  const arrow = graphArrowMaterial();
  const arrowMaterial = arrow.material;
  const arrowMesh = new THREE.LineSegments(arrows, arrowMaterial);
  arrowMesh.frustumCulled = false;
  arrowMesh.renderOrder = 1;
  scene.add(arrowMesh);
  let positions: Float32Array = new Float32Array(graph.nodes.length * 3);
  let visible: ReadonlySet<string> = index.connected;
  let selected = '';
  let path: readonly string[] = [];
  let active = true;
  let manual = false;
  let focusId = '';
  let initialized = false;
  let frame = 0;
  let hover = '';
  let styleDirty = true;
  let geometryDirty = true;
  const vector = new THREE.Vector3();

  function labels(): void {
    const projected: GraphLabelNode[] = [];
    const pathNodes = new Set(path);
    for (const id of visible) {
      const entry = index.nodes.get(id);
      if (!entry) continue;
      vector.fromArray(positions, entry.index * 3).applyMatrix4(camera.matrixWorldInverse);
      if (vector.z >= -camera.near) continue;
      vector.applyMatrix4(camera.projectionMatrix);
      if (Math.abs(vector.x) > 1 || Math.abs(vector.y) > 1 || Math.abs(vector.z) > 1) continue;
      projected.push({
        id,
        x: ((vector.x + 1) * host.clientWidth) / 2,
        y: ((1 - vector.y) * host.clientHeight) / 2,
        radius: graphNodeSize(points.sizes[entry.index] ?? 4, camera.zoom) / 2,
        priority:
          id === hover
            ? 4
            : id === selected
              ? 3
              : pathNodes.has(id)
                ? 2
                : id === graph.centerId
                  ? 1
                  : 0,
        persistent: id === hover || id === selected || id === graph.centerId,
      });
    }
    callbacks.labels(projected);
  }
  function updateBuffers(): void {
    const palette = graphPalette(host);
    const focus = hover || selected;
    const related = index.adjacent.get(focus);
    const steps = pathEdges(path);
    const pathNodes = new Set(path);
    scene.background = palette.background;
    points.uniforms.background.value.copy(palette.background);
    if (geometryDirty) {
      points.coordinates.set(positions);
      points.geometry.attributes.position.needsUpdate = true;
    }
    if (styleDirty) {
      graph.nodes.forEach((node, i) => {
        const emphasized = node.id === focus || pathNodes.has(node.id);
        points.sizes[i] = visible.has(node.id)
          ? emphasized
            ? 12
            : related?.has(node.id)
              ? 6
              : 4
          : 0;
        const base = emphasized
          ? palette.highlight
          : node.shortId
            ? palette.registered
            : palette.external;
        (focus && !emphasized && !related?.has(node.id)
          ? base.clone().lerp(palette.background, 0.45)
          : base
        ).toArray(points.colors, i * 3);
      });
      points.geometry.attributes.color.needsUpdate = true;
      points.geometry.attributes.size.needsUpdate = true;
    }
    edges.forEach((edge, i) => {
      const source = index.nodes.get(edge.source)?.index;
      const target = index.nodes.get(edge.target)?.index;
      if (source === undefined || target === undefined) return;
      const shown = visible.has(edge.source) && visible.has(edge.target);
      if (geometryDirty)
        for (let axis = 0; axis < 3; axis++) {
          coordinates[i * 6 + axis] = shown ? (positions[source * 3 + axis] ?? 0) : 0;
          coordinates[i * 6 + axis + 3] = shown ? (positions[target * 3 + axis] ?? 0) : 0;
        }
      if (styleDirty) {
        const emphasized = edge.source === focus || edge.target === focus;
        const onPath = steps.has(JSON.stringify([edge.source, edge.target]));
        const base = onPath
          ? palette.highlight
          : edge.reciprocal
            ? palette.reciprocal
            : palette.line;
        const strength = onPath ? 0.65 : emphasized ? 0.35 : focus ? 0.03 : 0;
        strengths[i * 2] = strength;
        strengths[i * 2 + 1] = strength;
        base.toArray(colors, i * 6);
        base.toArray(colors, i * 6 + 3);
      }
    });
    if (geometryDirty) lines.attributes.position.needsUpdate = true;
    if (styleDirty) {
      lines.attributes.color.needsUpdate = true;
      lines.attributes.strength.needsUpdate = true;
    }
    arrow.uniforms.color.value.copy(palette.highlight);
    const arrowGraph = {
      ...graph,
      edges: graph.edges.filter((edge) => visible.has(edge.source) && visible.has(edge.target)),
    };
    const nextArrows = graphArrows(arrowGraph, positions, focus, steps);
    arrowPositions.set(nextArrows);
    for (let offset = 0; offset < nextArrows.length; offset += 12) {
      for (let vertex = 0; vertex < 4; vertex++)
        arrowAnchors.set(nextArrows.subarray(offset, offset + 3), offset + vertex * 3);
    }
    if (nextArrows.length) {
      arrowPositionAttribute.addUpdateRange(0, nextArrows.length);
      arrowAnchorAttribute.addUpdateRange(0, nextArrows.length);
      arrowPositionAttribute.needsUpdate = true;
      arrowAnchorAttribute.needsUpdate = true;
    }
    arrows.setDrawRange(0, nextArrows.length / 3);
    geometryDirty = false;
    styleDirty = false;
  }
  function draw(): void {
    if (!active || document.hidden || frame) return;
    frame = requestAnimationFrame(() => {
      frame = 0;
      if (styleDirty || geometryDirty) updateBuffers();
      points.uniforms.zoom.value = camera.zoom;
      lineMaterial.uniforms.baseline.value = graphLineOpacity(camera.zoom);
      arrow.uniforms.height.value = host.clientHeight;
      renderer.render(scene, camera);
      labels();
    });
  }
  function fit(id?: string): void {
    const ids = id ? new Set([id, ...(index.adjacent.get(id) ?? [])]) : visible;
    const included = new Set(
      Array.from(ids).flatMap((key) => {
        const item = index.nodes.get(key);
        return item && visible.has(key) ? [item.index] : [];
      }),
    );
    const bounds = graphFrame(positions, camera.aspect, included);
    const direction = camera.position.clone().sub(controls.target).normalize();
    if (direction.lengthSq() === 0) direction.set(0, 0, 1);
    controls.target.fromArray(bounds.center);
    camera.position.copy(controls.target).addScaledVector(direction, bounds.distance);
    camera.zoom = 1;
    callbacks.zoom(1);
    camera.far = Math.max(100_000, bounds.distance * 16);
    camera.updateProjectionMatrix();
    controls.update();
    draw();
  }
  function reset(): void {
    manual = false;
    focusId = '';
    fit();
  }
  function hit(point: GraphPoint): string {
    if (!initialized) return '';
    let distance = 22;
    let front = Infinity;
    let id = '';
    for (const key of visible) {
      const item = index.nodes.get(key);
      if (!item) continue;
      vector.fromArray(positions, item.index * 3).project(camera);
      if (Math.abs(vector.z) > 1) continue;
      const next = Math.hypot(
        ((vector.x + 1) * canvas.clientWidth) / 2 - point.x,
        ((1 - vector.y) * canvas.clientHeight) / 2 - point.y,
      );
      if (next < distance || (next === distance && vector.z < front)) {
        distance = next;
        front = vector.z;
        id = key;
      }
    }
    return id;
  }
  function pick(point: GraphPoint, kind: 'hover' | 'select' | 'focus'): void {
    const id = hit(point);
    if (kind === 'hover') {
      if (hover !== id) {
        hover = id;
        styleDirty = true;
        draw();
      }
    } else if (id) {
      if (kind === 'focus') callbacks.focus(id);
      else callbacks.select(id);
    }
  }
  function leave(): void {
    if (!hover) return;
    hover = '';
    styleDirty = true;
    draw();
  }
  const input = createGraphControls(canvas, camera, {
    draw,
    manual: () => {
      manual = true;
      leave();
    },
    pick,
    leave,
    reset,
    zoomed: callbacks.zoom,
    depth: (point) => {
      const item = index.nodes.get(hit(point));
      if (!item) return undefined;
      return vector
        .fromArray(positions, item.index * 3)
        .sub(camera.position)
        .dot(camera.getWorldDirection(new THREE.Vector3()));
    },
    selected: () => {
      const item = index.nodes.get(selected);
      if (!item || !visible.has(selected)) return undefined;
      vector.fromArray(positions, item.index * 3).project(camera);
      return Math.abs(vector.x) <= 1 && Math.abs(vector.y) <= 1 && Math.abs(vector.z) <= 1
        ? {
            x: ((vector.x + 1) * canvas.clientWidth) / 2,
            y: ((1 - vector.y) * canvas.clientHeight) / 2,
          }
        : undefined;
    },
  });
  const { controls } = input;
  function resize(): void {
    if (!host.clientWidth || !host.clientHeight) return;
    renderer.setSize(host.clientWidth, host.clientHeight);
    camera.aspect = host.clientWidth / host.clientHeight;
    camera.updateProjectionMatrix();
    draw();
  }
  const observer = new ResizeObserver(resize);
  observer.observe(host);
  const theme = new MutationObserver(() => {
    styleDirty = true;
    draw();
  });
  theme.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
  const lost = (event: Event) => {
    event.preventDefault();
    host.dispatchEvent(new Event('graph-render-error'));
  };
  canvas.addEventListener('webglcontextlost', lost);
  document.addEventListener('visibilitychange', draw);
  resize();
  return {
    layout(next: Float32Array): void {
      const first = !initialized;
      positions = next;
      geometryDirty = true;
      initialized = true;
      // Input can arrive while the first Worker coordinates are still in flight.
      if (first || !manual) fit(focusId || undefined);
      draw();
    },
    select(id: string, nextPath: readonly string[], nextVisible: ReadonlySet<string>): void {
      selected = id;
      path = nextPath;
      if (visible !== nextVisible) geometryDirty = true;
      visible = nextVisible;
      styleDirty = true;
      draw();
    },
    configure(value: GraphInteraction): void {
      input.configure(value);
    },
    focus(id: string): void {
      if (index.nodes.has(id)) {
        manual = false;
        focusId = id;
        fit(id);
      }
    },
    zoom: input.zoom,
    reset,
    setActive(value: boolean): void {
      active = value;
      if (value) draw();
    },
    dispose(): void {
      observer.disconnect();
      theme.disconnect();
      input.dispose();
      cancelAnimationFrame(frame);
      document.removeEventListener('visibilitychange', draw);
      canvas.removeEventListener('webglcontextlost', lost);
      points.geometry.dispose();
      points.material.dispose();
      lines.dispose();
      lineMaterial.dispose();
      arrows.dispose();
      arrowMaterial.dispose();
      renderer.dispose();
      renderer.forceContextLoss();
      canvas.remove();
    },
  };
}
export type GraphRenderer = ReturnType<typeof createGraphRenderer>;
