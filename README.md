### Description

This program allows you to create custom cutscenes based off the hit gacha game, Blue Archive
all settings and systems are optimized for Blue Archive, but any sprites are possible.

### Instructions

Sprites, scripts, and animations are all dynamic

Inside ./resources/settings.json
you'll find some examples of how to setup sprite data, and how to create animations

### Hotkeys

You can enter debugging mode to help get the settings for sprites
press D to enter debugging mode
the character speaking will be in control
arrow keys: move around
s: shrink
b: get bigger

the position and scale of the character will show on screen, once you're happy with the values, put them into the settings.json file.

### Resources

Explanation of all the resources
(ignore shaders)

All files and folders should be lowercase and only contain letters, numbers, and underscores.

actor: 
here's where all the character sprites go, separate them by character name. All sprite files should be numbered with the character name in it.
for best results, sprites should be 1280x1280 in size
example:
```
resources \
  mika \
    mika-00.png
    mika-01.png
    mika-03.png
```

bg:
here's all the background images, these need to be 1920x1080 and .jpeg format
example:
```
resources \
  trinity_campus.jpeg
  trinity_fountain.jpeg
```

bgm:
all bgm music files here, must be .mp3 format
```
resources \
  theme_54.mp3 
```
to easily convert ogg to mp3, install a program called "ffmpeg", then run this command
```
ffmpeg -i <input>.<extension> <output>.<extension>
```

sfx: 
all sound effect files, in .mp3 format
```
resources \
  sfx_chat.mp3 
```

emote:
emotes are gifs in this system, all emotes should be gifs
```
resources \
  dots.gif
```

font:
at present, font is not configurable, but if you want, you can replace the font with your own and give it the same names as the files already there.

scripts:
at present, there's no easy way to choose the script you want to load. however all scripts should go in here as .txt files
if you want to load a specific script without renaming it, you can run this in the command line. Open this folder on your folder explorer, click file in the top right, open command line/powershell, then run this command,(replace <name> with the name of your script)
```
go run ./... -ldflags "-H windowsgui" <name>
```

ui:
not configurable but you can edit them if you want.

# Scripts

A script is compromised of two parts, an action and dialogue

an action can be many things, these are the supported formats:

A character speaking
characteR_name: can be any name, this name will be used to match any sprite settings defined in the settings.json file
expression: The expressions are considered numeric values based on the character name. (i.e. mika-01, mika-02)
animation: the name of the animation you want to play with this dialogue, the animations are defined in the settings.json file
[<character_name> - <expression> - <animation>]
<dialogue>

example:
```
[seia - 02 - move_left]
....Mika?
```

Sensei: you can add the little chat options for sensei's dialogue with this, supports up to two options
[sensei - _ - _]
<dialogue>
example:
```
[sensei - _ - _]
"I teach, therefore I am sensei"
```
or
```
[sensei - _ - _]
"This is your first option"
"And this is your second option"
```

You can use the word "all" to apply the command to all characters.
This will highlight all characters on screen, and apply any emotes, animations, and dialogue for all of them at once.
[all - _ - <animation/emote>]
<dialogue>
example:
```
[all - _ - _]
Good morning Sensei
```
another example:
```
[all - _ - dots]
```

this will change the background being used
[bg - <bg_name> - _]
example:
```
[bg - trinity_clubroom - _]
```

change the bgm playing:
NOTE: only mp3 files are supported currently
[bgm - <bgm_name> - _]
example:
```
[bgm - theme_54 - _]
```

play a sound effect just once
[sfx - <sfx_name> - _]
example:
```
[sfx - sfx_chat - _]
```

screen fade to black:
[fade - _ - in]
[fade - _ - out]

play an emote for the character instead of changing the expression
NOTE: emote settings are defined in the settings.json, mainly to dictate the position each emote should display on a characters. since every character might have a slightly different position
[<character_name> - <emote_name> - emote]
example:
```
[seia - dots - emote]
```
This is kind of a current limitation so the script will become a little bulky. If you want to change the character's expression AND play an emote at the same time you'll simply have to chain them together.
example:
```
[seia - 05 - move_right]
[seia - dots - emote]
```

at present, dialogue will only be considered on an action pertaining to a character.
You can stack actions in succession and then add dialogue anywhere in between, for example:
```
[bg - trinity_clubroom - none]
[seia - 00 - set_right]
[fade - _ - in]
[bgm - 54 - _]
[seia - 00 - move_left]
....Mika?
```

To remove all the sprites and dialogue from the scene, use
```
[clear - _ - _]
```

To remove only dialogue
```
[none - _ - _]
```

To change a character's faction during a script, use
```
[defect - <character_name> - _]
<new_faction_name>
```

Change the font size for dialogue, good for depicting screaming.
The font size will persist for all dialogue after it. So make sure to change it back when you're done
for reference, the default size is 0.85
[font - size - <size>]
example:
```
[font - size - 0.65]
```
and to bring it back to normal
[font - size - reset]

# Adding Animations

an animation is composed of multiple keyframes, you can add as many keyframes as you want
positioning for the sprites is based on the game's screen space normalized

the screen is setup as so
   ------( 1.0)-----
  |                 |
-1.0               1.0
  |                 |
   ------(-1.0)-----

use this as reference when making animations
```
{
  "name": "<animation_name>",
  "speed": 0.1, // a speed of 1 will make the animation end instantly
  "frames": [
      {
        "add_x": adds x amount to the current position of the character during the animation, 
        "add_y": same as above for the y position, 
        "x": sets the x position based on the screen,
        "y": same as x,
        "delay": amount of time to wait before playing the next frame IN SECONDS
        "reset": if true, all other options will be ignored and the character position is set to what it was before the animation played.
        "center": if true, ignore all other options, and set the position to the CENTER based on the center_x/center_y for this character.
      },
  ]
},
```
example:
```
{
    "name": "run_around",
    "speed": 0.05,
    "frames": [
        {"x": -0.5},
        {"center": true},
        {"x": 0.5},
        {"center": true},
        {"x": -0.5}
    ]
},
```

NOTE: if an animation is set to a speed of 1, it will complete instantly

### Troubleshooting

・if audio sounds weird, make sure the sample rate on the file is 44100
・if the app crashes, 99% of the time it's because something failed to load
  make sure to follow the instructions in the resources section to the letter.
  and properly convert any files if needed.