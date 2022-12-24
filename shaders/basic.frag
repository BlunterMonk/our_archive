#version 410 core

in vec3 ourColor;
in vec2 TexCoord;

out vec4 color;

uniform sampler2D ourTexture0;
uniform vec4 overlayColor; 

void main()
{
    // mix the two textures together (texture1 is colored with "ourColor")
    color = texture(ourTexture0, TexCoord) * overlayColor;
}
