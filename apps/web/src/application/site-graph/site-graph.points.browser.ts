import { BufferAttribute, BufferGeometry, Color, Points, ShaderMaterial } from 'three';

import type { SiteGraph } from '../../api/sites/site-graph.types.ts';

/** Screen-sized glyphs keep distant nodes legible without thousands of sphere meshes. */
export function createGraphPoints(graph: SiteGraph, pixelRatio: number) {
  const geometry = new BufferGeometry();
  const coordinates = new Float32Array(graph.nodes.length * 3);
  const colors = new Float32Array(coordinates.length);
  const sizes = new Float32Array(graph.nodes.length);
  geometry.setAttribute('position', new BufferAttribute(coordinates, 3));
  geometry.setAttribute('color', new BufferAttribute(colors, 3));
  geometry.setAttribute('size', new BufferAttribute(sizes, 1));
  geometry.setAttribute(
    'shape',
    new BufferAttribute(
      Float32Array.from(graph.nodes, (node) => (node.shortId ? 0 : 1)),
      1,
    ),
  );
  const uniforms = {
    pixelRatio: { value: pixelRatio },
    zoom: { value: 1 },
    background: { value: new Color() },
  };
  const material = new ShaderMaterial({
    uniforms,
    vertexColors: true,
    transparent: true,
    depthWrite: false,
    vertexShader: `
      attribute float size;
      attribute float shape;
      uniform float pixelRatio;
      uniform float zoom;
      varying vec3 nodeColor;
      varying float nodeShape;
      varying float nodeSize;
      void main() {
        vec4 view = modelViewMatrix * vec4(position, 1.0);
        nodeColor = color;
        nodeShape = shape;
        nodeSize = size;
        gl_Position = projectionMatrix * view;
        float cap = size >= 12.0 ? 28.0 : size >= 6.0 ? 22.0 : 18.0;
        gl_PointSize = min(cap, size * sqrt(max(1.0, zoom))) * pixelRatio;
      }
    `,
    fragmentShader: `
      varying vec3 nodeColor;
      varying float nodeShape;
      varying float nodeSize;
      uniform vec3 background;
      void main() {
        vec2 p = gl_PointCoord * 2.0 - 1.0;
        float distance = nodeShape < 0.5 ? length(p) : abs(p.x) + abs(p.y);
        if (nodeSize == 0.0 || distance > 1.0) discard;
        vec3 fill = mix(nodeColor, background, smoothstep(0.74, 0.9, distance) * 0.55);
        if (nodeSize >= 12.0 && distance > 0.65) {
          fill = distance < 0.82 ? background : nodeColor;
        }
        gl_FragColor = vec4(fill, 1.0 - smoothstep(0.9, 1.0, distance));
        #include <tonemapping_fragment>
        #include <colorspace_fragment>
      }
    `,
  });
  const mesh = new Points(geometry, material);
  mesh.renderOrder = 2;
  mesh.frustumCulled = false;
  return { mesh, geometry, material, uniforms, coordinates, colors, sizes };
}
