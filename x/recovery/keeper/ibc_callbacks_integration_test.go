package keeper_test

import (
	"time"

	"github.com/tomahawk-network/tomahawk/v6/app"
	"github.com/tomahawk-network/tomahawk/v6/x/recovery/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	channeltypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Recovery: Performing an IBC Transfer", Ordered, func() {
	cointomahawk := sdk.NewCoin("atoma", sdk.NewInt(10000))
	coinOsmo := sdk.NewCoin("uosmo", sdk.NewInt(10))
	coinAtom := sdk.NewCoin("uatom", sdk.NewInt(10))

	var (
		sender, receiver       string
		senderAcc, receiverAcc sdk.AccAddress
		timeout                uint64
		// claim                  claimtypes.ClaimsRecord
	)

	BeforeEach(func() {
		s.SetupTest()
	})

	Describe("from a non-authorized chain", func() {
		BeforeEach(func() {
			// params := "atoma"
			// params.AuthorizedChannels = []string{}

			sender = s.IBCOsmosisChain.SenderAccount.GetAddress().String()
			receiver = s.tomahawkChain.SenderAccount.GetAddress().String()
			senderAcc = sdk.MustAccAddressFromBech32(sender)
			receiverAcc = sdk.MustAccAddressFromBech32(receiver)
		})
		It("should transfer and not recover tokens", func() {
			s.SendAndReceiveMessage(s.pathOsmosistomahawk, s.IBCOsmosisChain, "uosmo", 10, sender, receiver, 1)

			nativetomahawk := s.tomahawkChain.App.(*app.Tomahawk).BankKeeper.GetBalance(s.tomahawkChain.GetContext(), senderAcc, "atoma")
			Expect(nativetomahawk).To(Equal(cointomahawk))
			ibcOsmo := s.tomahawkChain.App.(*app.Tomahawk).BankKeeper.GetBalance(s.tomahawkChain.GetContext(), receiverAcc, uosmoIbcdenom)
			Expect(ibcOsmo).To(Equal(sdk.NewCoin(uosmoIbcdenom, coinOsmo.Amount)))
		})
	})

	Describe("from an authorized, non-EVM chain (e.g. Osmosis)", func() {
		Describe("to a different account on tomahawk (sender != recipient)", func() {
			BeforeEach(func() {
				sender = s.IBCOsmosisChain.SenderAccount.GetAddress().String()
				receiver = s.tomahawkChain.SenderAccount.GetAddress().String()
				senderAcc = sdk.MustAccAddressFromBech32(sender)
				receiverAcc = sdk.MustAccAddressFromBech32(receiver)
			})

			It("should transfer and not recover tokens", func() {
				s.SendAndReceiveMessage(s.pathOsmosistomahawk, s.IBCOsmosisChain, "uosmo", 10, sender, receiver, 1)

				nativetomahawk := s.tomahawkChain.App.(*app.Tomahawk).BankKeeper.GetBalance(s.tomahawkChain.GetContext(), senderAcc, "atoma")
				Expect(nativetomahawk).To(Equal(cointomahawk))
				ibcOsmo := s.tomahawkChain.App.(*app.Tomahawk).BankKeeper.GetBalance(s.tomahawkChain.GetContext(), receiverAcc, uosmoIbcdenom)
				Expect(ibcOsmo).To(Equal(sdk.NewCoin(uosmoIbcdenom, coinOsmo.Amount)))
			})
		})

		Describe("to the sender's own eth_secp256k1 account on tomahawk (sender == recipient)", func() {
			BeforeEach(func() {
				sender = s.IBCOsmosisChain.SenderAccount.GetAddress().String()
				receiver = s.IBCOsmosisChain.SenderAccount.GetAddress().String()
				senderAcc = sdk.MustAccAddressFromBech32(sender)
				receiverAcc = sdk.MustAccAddressFromBech32(receiver)
			})

			Context("with disabled recovery parameter", func() {
				BeforeEach(func() {
					params := types.DefaultParams()
					params.EnableRecovery = false
					s.tomahawkChain.App.(*app.Tomahawk).RecoveryKeeper.SetParams(s.tomahawkChain.GetContext(), params)
				})

				It("should not transfer or recover tokens", func() {
					s.SendAndReceiveMessage(s.pathOsmosistomahawk, s.IBCOsmosisChain, coinOsmo.Denom, coinOsmo.Amount.Int64(), sender, receiver, 1)

					nativetomahawk := s.tomahawkChain.App.(*app.Tomahawk).BankKeeper.GetBalance(s.tomahawkChain.GetContext(), senderAcc, "atoma")
					Expect(nativetomahawk).To(Equal(cointomahawk))
					ibcOsmo := s.tomahawkChain.App.(*app.Tomahawk).BankKeeper.GetBalance(s.tomahawkChain.GetContext(), receiverAcc, uosmoIbcdenom)
					Expect(ibcOsmo).To(Equal(sdk.NewCoin(uosmoIbcdenom, coinOsmo.Amount)))
				})
			})

			Context("without a sender's claims record", func() {
				When("recipient has no ibc vouchers that originated from other chains", func() {
					It("should transfer and recover tokens", func() {
						// fmt.Println("Sender Account Numberc: ", s.IBCOsmosisChain.SenderAccount.GetAccountNumber())
						// fmt.Println("Sender Sequence: ", s.IBCOsmosisChain.SenderAccount.GetSequence())

						// atoma & ibc tokens that originated from the sender's chain
						s.SendAndReceiveMessage(s.pathOsmosistomahawk, s.IBCOsmosisChain, coinOsmo.Denom, coinOsmo.Amount.Int64(), sender, receiver, 1)
						timeout = uint64(s.tomahawkChain.GetContext().BlockTime().Add(time.Hour * 4).Add(time.Second * -20).UnixNano())

						// Escrow before relaying packets
						balanceEscrow := s.tomahawkChain.App.(*app.Tomahawk).BankKeeper.GetBalance(s.tomahawkChain.GetContext(), transfertypes.GetEscrowAddress("transfer", "channel-0"), "atoma")
						Expect(balanceEscrow).To(Equal(cointomahawk))
						ibcOsmo := s.tomahawkChain.App.(*app.Tomahawk).BankKeeper.GetBalance(s.tomahawkChain.GetContext(), receiverAcc, uosmoIbcdenom)
						Expect(ibcOsmo.IsZero()).To(BeTrue())

						// Relay both packets that were sent in the ibc_callback
						err := s.pathOsmosistomahawk.RelayPacket(CreatePacket("10000", "atoma", sender, receiver, "transfer", "channel-0", "transfer", "channel-0", 1, timeout))
						s.Require().NoError(err)
						err = s.pathOsmosistomahawk.RelayPacket(CreatePacket("10", "transfer/channel-0/uosmo", sender, receiver, "transfer", "channel-0", "transfer", "channel-0", 2, timeout))
						s.Require().NoError(err)

						// Check that the atoma were recovered
						nativetomahawk := s.tomahawkChain.App.(*app.Tomahawk).BankKeeper.GetBalance(s.tomahawkChain.GetContext(), senderAcc, "atoma")
						Expect(nativetomahawk.IsZero()).To(BeTrue())
						ibctomahawk := s.IBCOsmosisChain.GetSimApp().BankKeeper.GetBalance(s.IBCOsmosisChain.GetContext(), receiverAcc, atomaIbcdenom)
						Expect(ibctomahawk).To(Equal(sdk.NewCoin(atomaIbcdenom, cointomahawk.Amount)))

						// Check that the uosmo were recovered
						ibcOsmo = s.tomahawkChain.App.(*app.Tomahawk).BankKeeper.GetBalance(s.tomahawkChain.GetContext(), receiverAcc, uosmoIbcdenom)
						Expect(ibcOsmo.IsZero()).To(BeTrue())
						nativeOsmo := s.IBCOsmosisChain.GetSimApp().BankKeeper.GetBalance(s.IBCOsmosisChain.GetContext(), receiverAcc, "uosmo")
						Expect(nativeOsmo).To(Equal(coinOsmo))
					})
				})

				// Do not recover uatom sent from Cosmos when performing recovery through IBC transfer from Osmosis
				When("recipient has additional ibc vouchers that originated from other chains", func() {
					BeforeEach(func() {
						params := types.DefaultParams()
						params.EnableRecovery = false
						s.tomahawkChain.App.(*app.Tomahawk).RecoveryKeeper.SetParams(s.tomahawkChain.GetContext(), params)

						// Send uatom from Cosmos to tomahawk
						s.SendAndReceiveMessage(s.pathCosmostomahawk, s.IBCCosmosChain, coinAtom.Denom, coinAtom.Amount.Int64(), s.IBCCosmosChain.SenderAccount.GetAddress().String(), receiver, 1)

						params.EnableRecovery = true
						s.tomahawkChain.App.(*app.Tomahawk).RecoveryKeeper.SetParams(s.tomahawkChain.GetContext(), params)
					})
					It("should not recover tokens that originated from other chains", func() {
						// Send uosmo from Osmosis to tomahawk
						s.SendAndReceiveMessage(s.pathOsmosistomahawk, s.IBCOsmosisChain, "uosmo", 10, sender, receiver, 1)

						// Relay both packets that were sent in the ibc_callback
						timeout := uint64(s.tomahawkChain.GetContext().BlockTime().Add(time.Hour * 4).Add(time.Second * -20).UnixNano())
						err := s.pathOsmosistomahawk.RelayPacket(CreatePacket("10000", "atoma", sender, receiver, "transfer", "channel-0", "transfer", "channel-0", 1, timeout))
						s.Require().NoError(err)
						err = s.pathOsmosistomahawk.RelayPacket(CreatePacket("10", "transfer/channel-0/uosmo", sender, receiver, "transfer", "channel-0", "transfer", "channel-0", 2, timeout))
						s.Require().NoError(err)

						// Atomahawk was recovered from user address
						nativetomahawk := s.tomahawkChain.App.(*app.Tomahawk).BankKeeper.GetBalance(s.tomahawkChain.GetContext(), senderAcc, "atoma")
						Expect(nativetomahawk.IsZero()).To(BeTrue())
						ibctomahawk := s.IBCOsmosisChain.GetSimApp().BankKeeper.GetBalance(s.IBCOsmosisChain.GetContext(), receiverAcc, atomaIbcdenom)
						Expect(ibctomahawk).To(Equal(sdk.NewCoin(atomaIbcdenom, cointomahawk.Amount)))

						// Check that the uosmo were retrieved
						ibcOsmo := s.tomahawkChain.App.(*app.Tomahawk).BankKeeper.GetBalance(s.tomahawkChain.GetContext(), receiverAcc, uosmoIbcdenom)
						Expect(ibcOsmo.IsZero()).To(BeTrue())
						nativeOsmo := s.IBCOsmosisChain.GetSimApp().BankKeeper.GetBalance(s.IBCOsmosisChain.GetContext(), receiverAcc, "uosmo")
						Expect(nativeOsmo).To(Equal(coinOsmo))

						// Check that the atoms were not retrieved
						ibcAtom := s.tomahawkChain.App.(*app.Tomahawk).BankKeeper.GetBalance(s.tomahawkChain.GetContext(), senderAcc, uatomIbcdenom)
						Expect(ibcAtom).To(Equal(sdk.NewCoin(uatomIbcdenom, coinAtom.Amount)))

						// Repeat transaction from Osmosis to tomahawk
						s.SendAndReceiveMessage(s.pathOsmosistomahawk, s.IBCOsmosisChain, "uosmo", 10, sender, receiver, 2)

						timeout = uint64(s.tomahawkChain.GetContext().BlockTime().Add(time.Hour * 4).Add(time.Second * -20).UnixNano())
						err = s.pathOsmosistomahawk.RelayPacket(CreatePacket("10", "transfer/channel-0/uosmo", sender, receiver, "transfer", "channel-0", "transfer", "channel-0", 3, timeout))
						s.Require().NoError(err)

						// No further tokens recovered
						nativetomahawk = s.tomahawkChain.App.(*app.Tomahawk).BankKeeper.GetBalance(s.tomahawkChain.GetContext(), senderAcc, "atoma")
						Expect(nativetomahawk.IsZero()).To(BeTrue())
						ibctomahawk = s.IBCOsmosisChain.GetSimApp().BankKeeper.GetBalance(s.IBCOsmosisChain.GetContext(), receiverAcc, atomaIbcdenom)
						Expect(ibctomahawk).To(Equal(sdk.NewCoin(atomaIbcdenom, cointomahawk.Amount)))

						ibcOsmo = s.tomahawkChain.App.(*app.Tomahawk).BankKeeper.GetBalance(s.tomahawkChain.GetContext(), receiverAcc, uosmoIbcdenom)
						Expect(ibcOsmo.IsZero()).To(BeTrue())
						nativeOsmo = s.IBCOsmosisChain.GetSimApp().BankKeeper.GetBalance(s.IBCOsmosisChain.GetContext(), receiverAcc, "uosmo")
						Expect(nativeOsmo).To(Equal(coinOsmo))

						ibcAtom = s.tomahawkChain.App.(*app.Tomahawk).BankKeeper.GetBalance(s.tomahawkChain.GetContext(), senderAcc, uatomIbcdenom)
						Expect(ibcAtom).To(Equal(sdk.NewCoin(uatomIbcdenom, coinAtom.Amount)))
					})
				})

				// Recover ibc/uatom that was sent from Osmosis back to Osmosis
				When("recipient has additional non-native ibc vouchers that originated from senders chains", func() {
					BeforeEach(func() {
						params := types.DefaultParams()
						params.EnableRecovery = false
						s.tomahawkChain.App.(*app.Tomahawk).RecoveryKeeper.SetParams(s.tomahawkChain.GetContext(), params)

						s.SendAndReceiveMessage(s.pathOsmosisCosmos, s.IBCCosmosChain, coinAtom.Denom, coinAtom.Amount.Int64(), s.IBCCosmosChain.SenderAccount.GetAddress().String(), receiver, 1)

						// Send IBC transaction of 10 ibc/uatom
						transferMsg := transfertypes.NewMsgTransfer(s.pathOsmosistomahawk.EndpointA.ChannelConfig.PortID, s.pathOsmosistomahawk.EndpointA.ChannelID, sdk.NewCoin(uatomIbcdenom, sdk.NewInt(10)), sender, receiver, timeoutHeight, 0)
						_, err := s.IBCOsmosisChain.SendMsgs(transferMsg)
						s.Require().NoError(err) // message committed
						transfer := transfertypes.NewFungibleTokenPacketData("transfer/channel-1/uatom", "10", sender, receiver)
						packet := channeltypes.NewPacket(transfer.GetBytes(), 1, s.pathOsmosistomahawk.EndpointA.ChannelConfig.PortID, s.pathOsmosistomahawk.EndpointA.ChannelID, s.pathOsmosistomahawk.EndpointB.ChannelConfig.PortID, s.pathOsmosistomahawk.EndpointB.ChannelID, timeoutHeight, 0)
						// Receive message on the tomahawk side, and send ack
						err = s.pathOsmosistomahawk.RelayPacket(packet)
						s.Require().NoError(err)

						// Check that the ibc/uatom are available
						osmoIBCAtom := s.tomahawkChain.App.(*app.Tomahawk).BankKeeper.GetBalance(s.tomahawkChain.GetContext(), receiverAcc, uatomOsmoIbcdenom)
						s.Require().Equal(osmoIBCAtom.Amount, coinAtom.Amount)

						params.EnableRecovery = true
						s.tomahawkChain.App.(*app.Tomahawk).RecoveryKeeper.SetParams(s.tomahawkChain.GetContext(), params)
					})
					It("should not recover tokens that originated from other chains", func() {
						s.SendAndReceiveMessage(s.pathOsmosistomahawk, s.IBCOsmosisChain, "uosmo", 10, sender, receiver, 2)

						// Relay packets that were sent in the ibc_callback
						timeout := uint64(s.tomahawkChain.GetContext().BlockTime().Add(time.Hour * 4).Add(time.Second * -20).UnixNano())
						err := s.pathOsmosistomahawk.RelayPacket(CreatePacket("10000", "atoma", sender, receiver, "transfer", "channel-0", "transfer", "channel-0", 1, timeout))
						s.Require().NoError(err)
						err = s.pathOsmosistomahawk.RelayPacket(CreatePacket("10", "transfer/channel-0/transfer/channel-1/uatom", sender, receiver, "transfer", "channel-0", "transfer", "channel-0", 2, timeout))
						s.Require().NoError(err)
						err = s.pathOsmosistomahawk.RelayPacket(CreatePacket("10", "transfer/channel-0/uosmo", sender, receiver, "transfer", "channel-0", "transfer", "channel-0", 3, timeout))
						s.Require().NoError(err)

						// Atomahawk was recovered from user address
						nativetomahawk := s.tomahawkChain.App.(*app.Tomahawk).BankKeeper.GetBalance(s.tomahawkChain.GetContext(), senderAcc, "atoma")
						Expect(nativetomahawk.IsZero()).To(BeTrue())
						ibctomahawk := s.IBCOsmosisChain.GetSimApp().BankKeeper.GetBalance(s.IBCOsmosisChain.GetContext(), receiverAcc, atomaIbcdenom)
						Expect(ibctomahawk).To(Equal(sdk.NewCoin(atomaIbcdenom, cointomahawk.Amount)))

						// Check that the uosmo were recovered
						ibcOsmo := s.tomahawkChain.App.(*app.Tomahawk).BankKeeper.GetBalance(s.tomahawkChain.GetContext(), receiverAcc, uosmoIbcdenom)
						Expect(ibcOsmo.IsZero()).To(BeTrue())
						nativeOsmo := s.IBCOsmosisChain.GetSimApp().BankKeeper.GetBalance(s.IBCOsmosisChain.GetContext(), receiverAcc, "uosmo")
						Expect(nativeOsmo).To(Equal(coinOsmo))

						// Check that the ibc/uatom were retrieved
						osmoIBCAtom := s.tomahawkChain.App.(*app.Tomahawk).BankKeeper.GetBalance(s.tomahawkChain.GetContext(), receiverAcc, uatomOsmoIbcdenom)
						Expect(osmoIBCAtom.IsZero()).To(BeTrue())
						ibcAtom := s.IBCOsmosisChain.GetSimApp().BankKeeper.GetBalance(s.IBCOsmosisChain.GetContext(), senderAcc, uatomIbcdenom)
						Expect(ibcAtom).To(Equal(sdk.NewCoin(uatomIbcdenom, sdk.NewInt(10))))
					})
				})
			})
		})
	})
})
