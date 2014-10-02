// Copyright 2014 Eric Holmes.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package chip8 provides a Go implementation of the CHIP-8 emulator.
//
// CHIP-8 was most commonly implemented on 4K systems, such as the
// Cosmac VIP and the Telemac 1800. These machines had 4096 (0x1000)
// memory locations, all of which are 8 bits (a byte) which is where the
// term CHIP-8 originated. However, the CHIP-8 interpreter itself
// occupies the first 512 bytes of the memory space on these machines.
// For this reason, most programs written for the original system begin
// at memory location 512 (0x200) and do not access any of the memory
// below the location 512 (0x200). The uppermost 256 bytes (0xF00-0xFFF)
// are reserved for display refresh, and the 96 bytes below that
// (0xEA0-0XEFF) were reserved for call stack, internal use, and other
// variables.
package chip8

import (
	"fmt"
	"time"
)

// Sensible defaults
var (
	DefaultClockSpeed = time.Duration(60) // Hz
	DefaultOptions    = &Options{
		ClockSpeed: DefaultClockSpeed,
	}
)

// CPU represents a CHIP-8 CPU.
type CPU struct {
	// The 4096 bytes of memory.
	//
	// Memory Map:
	// +---------------+= 0xFFF (4095) End of Chip-8 RAM
	// |               |
	// |               |
	// |               |
	// |               |
	// |               |
	// | 0x200 to 0xFFF|
	// |     Chip-8    |
	// | Program / Data|
	// |     Space     |
	// |               |
	// |               |
	// |               |
	// +- - - - - - - -+= 0x600 (1536) Start of ETI 660 Chip-8 programs
	// |               |
	// |               |
	// |               |
	// +---------------+= 0x200 (512) Start of most Chip-8 programs
	// | 0x000 to 0x1FF|
	// | Reserved for  |
	// |  interpreter  |
	// +---------------+= 0x000 (0) Start of Chip-8 RAM
	Memory [4096]byte

	// The address register, which is named I, is 16 bits wide and is used
	// with several opcodes that involve memory operations.
	I uint16

	// Program counter.
	PC uint16

	// CHIP-8 has 16 8-bit data registers named from V0 to VF. The VF
	// register doubles as a carry flag.
	V [16]byte

	// The stack is only used to store return addresses when subroutines are
	// called. The original 1802 version allocated 48 bytes for up to 12
	// levels of nesting; modern implementations normally have at least 16
	// levels.
	Stack [16]uint16

	// Stack pointer.
	SP byte

	// The CHIP-8 timers count down at 60 Hz, so we slow down the cpu clock
	// to only execute 60 opcodes per second.
	Clock <-chan time.Time
}

// Options provides a means of configuring the CPU.
type Options struct {
	ClockSpeed time.Duration
}

// NewCPU returns a new CPU instance.
func NewCPU(options *Options) *CPU {
	if options == nil {
		options = DefaultOptions
	}

	return &CPU{
		PC:    200,
		Clock: time.Tick(time.Second / options.ClockSpeed),
	}
}

// Step runs a single CPU cycle.
func (c *CPU) Step() error {
	// Simulate the clock speed of the CHIP-8 CPU.
	<-c.Clock

	// Dispatch the opcode.
	if err := c.Dispatch(c.op()); err != nil {
		return err
	}

	return nil
}

// Dispatch executes the given opcode.
func (c *CPU) Dispatch(op uint16) error {
	// In these listings, the following variables are used:
	//
	// nnn or addr - A 12-bit value, the lowest 12 bits of the instruction
	// n or nibble - A 4-bit value, the lowest 4 bits of the instruction
	// x - A 4-bit value, the lower 4 bits of the high byte of the instruction
	// y - A 4-bit value, the upper 4 bits of the low byte of the instruction
	// kk or byte - An 8-bit value, the lowest 8 bits of the instruction

	switch op & 0xF000 {
	// 0nnn - SYS addr
	case 0x0000:
		switch op {
		// 00E0 - CLS
		case 0x00E0:
			// TODO
			break

		// 00EE - RET
		case 0x00EE:
			// Return from a subroutine.
			//
			// The interpreter sets the program counter to the
			// address at the top of the stack, then subtracts 1
			// from the stack pointer.

			c.PC = c.Stack[c.SP]
			c.SP--

			break

		default:
			// Jump to a machine code routine at nnn.
			//
			// This instruction is only used on the old computers on
			// which Chip-8 was originally implemented. It is
			// ignored by modern interpreters.
		}

	// 1nnn - JP addr
	case 0x1000:
		// Jump to location nnn.
		//
		// The interpreter sets the program counter to nnn.

		c.PC = op & 0x0FFF

		break

	// 2nnn - CALL addr
	case 0x2000:
		// Call subroutine at nnn.
		//
		// The interpreter increments the stack pointer, then puts the
		// current PC on the top of the stack. The PC is then set to
		// nnn.

		c.Stack[c.SP] = c.PC
		c.SP++
		c.PC = op & 0x0FFF

		break

	// 3xkk - SE Vx, byte
	case 0x3000:
		// Skip next instruction if Vx = kk.

		// The interpreter compares register Vx to kk, and if they are
		// equal, increments the program counter by 2.

		x := (op & 0x0F00) >> 8
		kk := byte(op & 0x000F)

		if c.V[x] == kk {
			c.PC += 2
		}

		break

	// 4xkk - SNE Vx, byte
	case 0x4000:
		// Skip next instruction if Vx != kk.
		//
		// The interpreter compares register Vx to kk, and if they are
		// not equal, increments the program counter by 2.

		x := (op & 0x0F00) >> 8
		kk := byte(op & 0x000F)

		if c.V[x] != kk {
			c.PC += 2
		}

		break

	// 5xy0 - SE Vx, Vy
	case 0x5000:
		switch op & 0xF00F {
		case 0x5000:
			// Skip next instruction if Vx = Vy.
			//
			// The interpreter compares register Vx to register Vy, and if
			// they are equal, increments the program counter by 2.

			x := (op & 0x0F00) >> 8
			y := (op & 0x00F0) >> 4

			if c.V[x] == c.V[y] {
				c.PC += 2
			}

			break
		default:
			return &UnknownOpcode{Opcode: op}
		}

		break

	// 6xkk - LD Vx, byte
	case 0x6000:
		// Set Vx = kk.
		//
		// The interpreter puts the value kk into register Vx.

		x := (op & 0x0F00) >> 8
		kk := byte(op & 0x000F)

		c.V[x] = kk

		break

	// Adds NN to VX
	//   0x7XNN
	case 0x7000:

	case 0x8000:

		switch op & 0x000F {
		// Sets VX to the value of VY.
		case 0x00:

		// Sets VX to VX or VY.
		case 0x01:

		// Sets VX to VX and VY.
		case 0x02:

		// Sets VX to VX xor VY.
		case 0x03:

		// Adds VY to VX. VF is set to 1 when there's a carry, and to 0 when there isn't.
		case 0x04:

		// VY is subtracted from VX. VF is set to 0 when there's a borrow, and 1 when there isn't.
		case 0x05:
		// Shifts VX right by one. VF is set to the value of the least significant bit of VX before the shift.
		case 0x06:
		// Sets VX to VY minus VX. VF is set to 0 when there's a borrow, and 1 when there isn't.
		case 0x07:
		// Shifts VX left by one. VF is set to the value of the most significant bit of VX before the shift.
		case 0x0E:
		}

	// Skips the next instruction if VX doesn't equal VY.
	//   0x9XY0
	case 0x9000:

	// Sets I to the address NNN.
	//   0xANNN
	case 0xA000:
		c.I = op & 0x0FFF
		c.PC += 2
		break

	// Jumps to the address NNN plus V0.
	//   0xBNNN
	case 0xB000:

	// Sets VX to a random number and NN.
	//   0xCXNN
	case 0xC000:

	// Draws a sprite at coordinate (VX, VY) that has a width of 8 pixels and a
	// height of N pixels. Each row of 8 pixels is read as bit-coded (with the
	// most significant bit of each byte displayed on the left) starting from
	// memory location I; I value doesn't change after the execution of this
	// instruction. As described above, VF is set to 1 if any screen pixels are
	// flipped from set to unset when the sprite is drawn, and to 0 if that doesn't
	// happen.
	//
	//   0xDXYN
	case 0xD000:

	case 0xE000:
		switch op & 0x00FF {
		// Skips the next instruction if the key stored in VX is pressed.
		case 0x9E:

		// Skips the next instruction if the key stored in VX isn't pressed.
		case 0xA1:
		}
	case 0xF000:
		switch op & 0x00FF {
		// Sets VX to the value of the delay timer.
		case 0x07:

		// A key press is awaited, and then stored in VX.
		case 0x0A:

		// Sets the delay timer to VX.
		case 0x15:

		// Sets the sound timer to VX.
		case 0x18:

		// Adds VX to I.
		case 0x1E:

		// Sets I to the location of the sprite for the character in VX. Characters
		// 0-F (in hexadecimal) are represented by a 4x5 font.
		case 0x29:

		// Stores the Binary-coded decimal representation of VX, with the most
		// significant of three digits at the address in I, the middle digit at
		// I plus 1, and the least significant digit at I plus 2. (In other words,
		// take the decimal representation of VX, place the hundreds digit in
		// memory at location in I, the tens digit at location I+1, and the ones
		// digit at location I+2.)
		case 0x33:

		// Stores V0 to VX in memory starting at address I.
		case 0x55:

		// Fills V0 to VX with values from memory starting at address I.
		case 0x65:
		}
	default:
		return &UnknownOpcode{Opcode: op}
	}

	return nil
}

// op returns the next op code.
func (c *CPU) op() uint16 {
	return uint16(c.Memory[c.PC])<<8 | uint16(c.Memory[c.PC+1])
}

// UnknownOpcode is return when the opcode is not recognized.
type UnknownOpcode struct {
	Opcode uint16
}

func (e *UnknownOpcode) Error() string {
	return fmt.Sprintf("chip8: unknown opcode: 0x%04X", e.Opcode)
}
