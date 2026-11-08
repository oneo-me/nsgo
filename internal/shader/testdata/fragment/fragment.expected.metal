#include <metal_stdlib>

using namespace metal;

constexpr sampler __texelSampler(coord::pixel, filter::nearest, address::clamp_to_zero);

template<typename T, typename U>
T mod(T x, U y) {
	return x - y * floor(x/y);
}

// Metal defines sign only for floating-point types. These overloads cover int and ivecN arguments.
int sign(int x) {
	return int(x > 0) - int(x < 0);
}

int2 sign(int2 x) {
	return int2(x > 0) - int2(x < 0);
}

int3 sign(int3 x) {
	return int3(x > 0) - int3(x < 0);
}

int4 sign(int4 x) {
	return int4(x > 0) - int4(x < 0);
}

// WGSL's bitcasts (package shader/wgsl), by width.
int as_type_int(float x) { return as_type<int>(x); }
int2 as_type_int(float2 x) { return as_type<int2>(x); }
int3 as_type_int(float3 x) { return as_type<int3>(x); }
int4 as_type_int(float4 x) { return as_type<int4>(x); }
uint as_type_uint(float x) { return as_type<uint>(x); }
uint2 as_type_uint(float2 x) { return as_type<uint2>(x); }
uint3 as_type_uint(float3 x) { return as_type<uint3>(x); }
uint4 as_type_uint(float4 x) { return as_type<uint4>(x); }
float as_type_float(int x) { return as_type<float>(x); }
float2 as_type_float(int2 x) { return as_type<float2>(x); }
float3 as_type_float(int3 x) { return as_type<float3>(x); }
float4 as_type_float(int4 x) { return as_type<float4>(x); }
float as_type_float(uint x) { return as_type<float>(x); }
float2 as_type_float(uint2 x) { return as_type<float2>(x); }
float3 as_type_float(uint3 x) { return as_type<float3>(x); }
float4 as_type_float(uint4 x) { return as_type<float4>(x); }

struct Uniforms {
	float2 U0;
	float U1;
	uint U2;
	int U3;
	array<float4, 4> U4;
};

uint F0(constant Uniforms& uniforms, uint l0);
float2 F1(constant Uniforms& uniforms);
float F2(constant Uniforms& uniforms);
float4 F3(constant Uniforms& uniforms, float4 l0, float2 l1);

uint F0(constant Uniforms& uniforms, uint l0) {
	uint l1 = 0u;
	uint l2 = 0u;
	l1 = ((l0) * (747796405u)) + (2891336453u);
	l2 = (((l1) >> (((l1) >> (28)) + (4u))) ^ (l1)) * (277803737u);
	return ((l2) >> (22)) ^ (l2);
}

float2 F1(constant Uniforms& uniforms) {
	return uniforms.U0;
}

float F2(constant Uniforms& uniforms) {
	return uniforms.U1;
}

float4 F3(constant Uniforms& uniforms, float4 l0, float2 l1) {
	float l2 = float(0);
	uint l5 = 0u;
	l2 = 0.0;
	for (int l3 = 0; l3 < uniforms.U3; l3++) {
		l2 = (l2) + ((((uniforms.U4)[l3]).z) / ((1.0) + (distance(l1, ((uniforms.U4)[l3]).xy))));
	}
	for (uint l4 = 0u; l4 < uniforms.U2; l4 += 2u) {
		l2 = (l2) + (1.0000000000e-01);
	}
	l5 = F0(uniforms, (((uint3(static_cast<uint>(static_cast<int>((l0).x)))).z) * (2654435769u)) ^ (F0(uniforms, (static_cast<uint>(static_cast<int>((l0).y))) ^ (uniforms.U2))));
	return (((float4(((l1) * (F2(uniforms))) / (F1(uniforms)), 0.0, 1.0)) * (static_cast<float>((l5) % (255u)))) / (255.0)) * (l2);
}

float4 shaderFragment(constant Uniforms& uniforms, float4 position, float2 v0) {
	return F3(uniforms, position, v0);
}
