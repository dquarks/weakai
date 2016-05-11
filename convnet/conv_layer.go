package convnet

import (
	"math"
	"math/rand"

	"github.com/unixpickle/num-analysis/kahan"
)

// ConvParams stores parameters that define
// a convolutional layer in an ANN.
// It can be used as a LayerPrototype to make
// convolutional layers.
type ConvParams struct {
	FilterCount  int
	FilterWidth  int
	FilterHeight int
	Stride       int

	InputWidth  int
	InputHeight int
	InputDepth  int

	Activation ActivationFunc
}

// Make creates a new *ConvLayer using the
// options specified in p.
// This is equivalent to NewConvLayer(p).
func (p *ConvParams) Make() Layer {
	return NewConvLayer(p)
}

// A ConvLayer serves as a convolutional layer
// in a neural network.
type ConvLayer struct {
	tensorLayer

	activation ActivationFunc
	stride     int

	filters         []*Tensor3
	filterGradients []*Tensor3
	biases          []float64
	biasPartials    []float64

	convolutions *Tensor3
}

// NewConvLayer creates a *ConvLayer using the
// specified parameters.
//
// The resulting layer will be filled with zero
// weights, biases, and outputs.
// It will have a nil input and downstream gradient.
func NewConvLayer(params *ConvParams) *ConvLayer {
	w := 1 + (params.InputWidth-params.FilterWidth)/params.Stride
	h := 1 + (params.InputHeight-params.FilterHeight)/params.Stride

	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}

	res := &ConvLayer{
		tensorLayer: tensorLayer{
			output:           NewTensor3(w, h, params.FilterCount),
			upstreamGradient: NewTensor3(params.InputWidth, params.InputHeight, params.InputDepth),
		},

		activation: params.Activation,
		stride:     params.Stride,

		filters:         make([]*Tensor3, params.FilterCount),
		filterGradients: make([]*Tensor3, params.FilterCount),
		biases:          make([]float64, params.FilterCount),
		biasPartials:    make([]float64, params.FilterCount),

		convolutions: NewTensor3(w, h, params.FilterCount),
	}

	for i := 0; i < params.FilterCount; i++ {
		res.filters[i] = NewTensor3(params.FilterWidth, params.FilterHeight, params.InputDepth)
		res.filterGradients[i] = NewTensor3(params.FilterWidth, params.FilterHeight,
			params.InputDepth)
	}

	return res
}

func (c *ConvLayer) Randomize() {
	for i, filter := range c.filters {
		filter.Randomize()
		c.biases[i] = rand.Float64()
	}
}

func (c *ConvLayer) PropagateForward() {
	for y := 0; y < c.output.Height; y++ {
		inputY := y * c.stride
		for x := 0; x < c.output.Width; x++ {
			inputX := x * c.stride
			for z, filter := range c.filters {
				convolution := filter.Convolve(inputX, inputY, filter)
				convolution += c.biases[z]
				c.convolutions.Set(x, y, z, convolution)
				c.output.Set(x, y, z, c.activation.Eval(convolution))
			}
		}
	}
}

func (c *ConvLayer) PropagateBackward() {
	for i, x := range c.filterGradients {
		x.Reset()
		c.biasPartials[i] = 0
	}
	c.upstreamGradient.Reset()

	for y := 0; y < c.output.Height; y++ {
		inputY := y * c.stride
		for x := 0; x < c.output.Width; x++ {
			inputX := x * c.stride
			for z, filter := range c.filters {
				sumPartial := c.downstreamGradient.Get(x, y, z) *
					c.activation.Deriv(c.convolutions.Get(x, y, z))
				c.filterGradients[z].MulAdd(-inputX, -inputY, c.input, sumPartial)
				c.upstreamGradient.MulAdd(inputX, inputY, filter, sumPartial)
				c.biasPartials[z] += sumPartial
			}
		}
	}
}

func (c *ConvLayer) GradientMagSquared() float64 {
	sum := kahan.NewSummer64()

	for i, filterGrad := range c.filterGradients {
		sum.Add(math.Pow(c.biasPartials[i], 2))
		for _, val := range filterGrad.Data {
			sum.Add(val * val)
		}
	}

	return sum.Sum()
}

func (c *ConvLayer) StepGradient(f float64) {
	for i, filterGrad := range c.filterGradients {
		c.biases[i] += c.biasPartials[i] * f
		for j, val := range filterGrad.Data {
			c.filters[i].Data[j] += val * f
		}
	}
}
