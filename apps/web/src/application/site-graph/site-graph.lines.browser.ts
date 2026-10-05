import { Color, ShaderMaterial } from 'three';

export function graphArrowMaterial() {
  const uniforms = { color: { value: new Color() }, height: { value: 1 } };
  const material = new ShaderMaterial({
    uniforms,
    transparent: true,
    depthWrite: false,
    vertexShader: `
      attribute vec3 anchor;
      uniform float height;
      void main() {
        vec4 origin = modelViewMatrix * vec4(anchor, 1.0);
        float unit = 2.0 * max(0.1, -origin.z) / (projectionMatrix[1][1] * height);
        gl_Position = projectionMatrix * modelViewMatrix * vec4(anchor + (position - anchor) * unit, 1.0);
      }
    `,
    fragmentShader: `
      uniform vec3 color;
      void main() {
        gl_FragColor = vec4(color, 0.45);
        #include <tonemapping_fragment>
        #include <colorspace_fragment>
      }
    `,
  });
  return { material, uniforms };
}

/** Baseline fading is a uniform so zoom never rebuilds the full edge buffer. */
export function graphLineMaterial(): ShaderMaterial {
  return new ShaderMaterial({
    uniforms: { baseline: { value: 0.06 } },
    vertexColors: true,
    transparent: true,
    depthWrite: false,
    vertexShader: `
      attribute float strength;
      varying vec3 edgeColor;
      varying float edgeStrength;
      void main() {
        edgeColor = color;
        edgeStrength = strength;
        gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
      }
    `,
    fragmentShader: `
      uniform float baseline;
      varying vec3 edgeColor;
      varying float edgeStrength;
      void main() {
        gl_FragColor = vec4(edgeColor, edgeStrength == 0.0 ? baseline : edgeStrength);
        #include <tonemapping_fragment>
        #include <colorspace_fragment>
      }
    `,
  });
}
