#version 410 core

layout (location = 0) in vec3 position;
layout (location = 1) in vec3 color;
layout (location = 2) in vec2 texCoord;

uniform mat4 prjMatrix;
//mat4 prjMatrix = mat4(
//    1, 0, 0, 0,
//	0, 1, 0, 0,
//	0, 0, 1, 0,
//	0, 0, 0, 1
//);

out vec3 ourColor;
out vec2 TexCoord;

void main()
{
    gl_Position = vec4(position, 1.0) * prjMatrix;
    ourColor = color;       // pass the color on to the fragment shader
    TexCoord = texCoord;    // pass the texture coords on to the fragment shader
}
