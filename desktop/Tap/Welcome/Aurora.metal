// The tap.sh northern lights (tap-sh/public/landing/aurora.js), ported from
// GLSL: three drifting curtains of simplex-noise rays, tinted along a
// four-stop green ramp, premultiplied so they composite over any background.
// The welcome window draws this at half resolution and blurs it in two
// separable passes.
#include <metal_stdlib>
using namespace metal;

struct AuroraUniforms {
    float4 frame;   // resolution.x, resolution.y, time, intensity
    float4 shape;   // height, unused, unused, unused
    float4 colors[4];
};

struct BlurUniforms {
    float4 step;    // texel step x, texel step y, sigma in texels, unused
};

struct VertexOut {
    float4 position [[position]];
    float2 uv;
};

vertex VertexOut auroraVertex(uint vertexID [[vertex_id]]) {
    float2 corners[4] = { float2(-1, -1), float2(1, -1), float2(-1, 1), float2(1, 1) };
    VertexOut out;
    out.position = float4(corners[vertexID], 0, 1);
    out.uv = float2(corners[vertexID].x * 0.5 + 0.5, 0.5 - corners[vertexID].y * 0.5);
    return out;
}

static float3 mod289(float3 x) { return x - floor(x * (1.0 / 289.0)) * 289.0; }
static float2 mod289(float2 x) { return x - floor(x * (1.0 / 289.0)) * 289.0; }
static float3 permute(float3 x) { return mod289(((x * 34.0) + 1.0) * x); }

static float snoise(float2 v) {
    const float4 C = float4(0.211324865405187, 0.366025403784439, -0.577350269189626, 0.024390243902439);
    float2 i = floor(v + dot(v, C.yy));
    float2 x0 = v - i + dot(i, C.xx);
    float2 i1 = (x0.x > x0.y) ? float2(1.0, 0.0) : float2(0.0, 1.0);
    float4 x12 = x0.xyxy + C.xxzz;
    x12.xy -= i1;
    i = mod289(i);
    float3 p = permute(permute(i.y + float3(0.0, i1.y, 1.0)) + i.x + float3(0.0, i1.x, 1.0));
    float3 m = max(0.5 - float3(dot(x0, x0), dot(x12.xy, x12.xy), dot(x12.zw, x12.zw)), 0.0);
    m = m * m;
    m = m * m;
    float3 x = 2.0 * fract(p * C.www) - 1.0;
    float3 h = abs(x) - 0.5;
    float3 ox = floor(x + 0.5);
    float3 a0 = x - ox;
    m *= 1.79284291400159 - 0.85373472095314 * (a0 * a0 + h * h);
    float3 g;
    g.x = a0.x * x0.x + h.x * x0.y;
    g.yz = a0.yz * x12.xz + h.yz * x12.yw;
    return 130.0 * dot(m, g);
}

static float fbm(float2 p) {
    float v = 0.0, a = 0.5;
    for (int i = 0; i < 5; i++) {
        v += a * snoise(p);
        p = p * 2.02 + float2(17.0, 9.0);
        a *= 0.5;
    }
    return v;
}

static float3 ramp(float h, constant AuroraUniforms &u) {
    float3 c = mix(u.colors[0].rgb, u.colors[1].rgb, smoothstep(0.0, 0.45, h));
    c = mix(c, u.colors[2].rgb, smoothstep(0.35, 0.8, h));
    c = mix(c, u.colors[3].rgb, smoothstep(0.75, 1.0, h) * 0.7);
    return c;
}

fragment half4 auroraFragment(VertexOut in [[stage_in]], constant AuroraUniforms &u [[buffer(0)]]) {
    float2 uv = in.uv; // origin at the bottom left, as the WebGL original has it
    uv.y = 1.0 - uv.y;
    float aspect = u.frame.x / u.frame.y;
    float t = u.frame.z;
    float intensity = u.frame.w;
    float height = u.shape.x;
    float3 col = float3(0.0);
    float2 s = float2(uv.x * aspect, uv.y);
    float acc = 0.0;
    for (int i = 0; i < 3; i++) {
        float fi = float(i);
        float base = -0.12 + fi * 0.16 + 0.05 * snoise(float2(s.x * 0.6 + fi * 3.1, t * 0.05 + fi));
        float rays = fbm(float2(s.x * (3.5 + fi * 1.5) + t * (0.05 + fi * 0.02), fi * 7.0)) * 0.5 + 0.5;
        rays = pow(rays, 1.3);
        float fine = snoise(float2(s.x * (18.0 + fi * 6.0) + t * 0.12, fi * 3.0)) * 0.5 + 0.5;
        rays *= 0.75 + 0.25 * fine;
        float dy = s.y - base;
        float span = 0.9 + height * 0.6;
        float profile = smoothstep(-0.02, 0.06, dy) * exp(-max(dy, 0.0) * (1.6 / span));
        float flicker = 0.85 + 0.15 * snoise(float2(s.x * 2.0 + t * 0.3, t * 0.2 + fi));
        float v = rays * profile * flicker * (1.0 - fi * 0.2);
        col += ramp(clamp(dy / span, 0.0, 1.0), u) * v;
        acc += v;
    }
    col = col / max(acc, 1e-3);
    float a = clamp(acc * 1.1, 0.0, 1.0);
    float edgeX = smoothstep(0.0, 0.12, uv.x) * smoothstep(1.0, 0.88, uv.x);
    float edgeY = smoothstep(1.0, 0.8, uv.y) * smoothstep(0.0, 0.1, uv.y);
    float alpha = clamp(a * edgeX * edgeY * intensity, 0.0, 1.0);
    return half4(half3(col * alpha), half(alpha));
}

// One separable Gaussian pass over a premultiplied image.
static half4 blurred(VertexOut in, texture2d<half> source, constant BlurUniforms &u) {
    constexpr sampler linearClamp(coord::normalized, address::clamp_to_edge, filter::linear);
    float sigma = max(u.step.z, 0.001);
    int radius = int(ceil(sigma * 3.0));
    half4 sum = half4(0.0);
    float total = 0.0;
    for (int i = -radius; i <= radius; i++) {
        float weight = exp(-0.5 * float(i * i) / (sigma * sigma));
        sum += source.sample(linearClamp, in.uv + u.step.xy * float(i)) * half(weight);
        total += weight;
    }
    return sum / half(total);
}

fragment half4 auroraBlurFragment(VertexOut in [[stage_in]],
                                  texture2d<half> source [[texture(0)]],
                                  constant BlurUniforms &u [[buffer(0)]]) {
    return blurred(in, source, u);
}

// The last blur pass adds a sub-step of noise so the slow gradients do not band in 8 bits.
fragment half4 auroraFinalBlurFragment(VertexOut in [[stage_in]],
                                       texture2d<half> source [[texture(0)]],
                                       constant BlurUniforms &u [[buffer(0)]]) {
    half4 color = blurred(in, source, u);
    float noise = fract(sin(dot(in.position.xy, float2(12.9898, 78.233))) * 43758.5453) - 0.5;
    half dither = half(noise / 255.0);
    half alpha = clamp(color.a + dither * step(0.0005h, color.a), 0.0h, 1.0h);
    half3 rgb = clamp(color.rgb + dither * step(0.0005h, color.a), 0.0h, alpha);
    return half4(rgb, alpha);
}
