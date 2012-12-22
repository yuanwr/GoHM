package TLibDecoder

import (
	//"container/list"
    "gohm/TLibCommon"
)

/// slice decoder class
type TDecSlice struct {
    //private:
    // access channel
    m_pcEntropyDecoder				*TDecEntropy;
    m_pcCuDecoder					*TDecCu;
    m_uiCurrSliceIdx 				uint

    m_pcBufferSbacDecoders			[]*TDecSbac;   ///< line to store temporary contexts, one per column of tiles.
    m_pcBufferBinCABACs				[]*TDecBinCabac;
    m_pcBufferLowLatSbacDecoders	[]*TDecSbac;   ///< dependent tiles: line to store temporary contexts, one per column of tiles.
    m_pcBufferLowLatBinCABACs		[]*TDecBinCabac;
    //#if DEPENDENT_SLICES
    CTXMem							map[int]*TDecSbac;//*list.List;//std::vector<TDecSbac*> 
    //#endif
}

//public:
func NewTDecSlice() *TDecSlice {
    return &TDecSlice{CTXMem:make(map[int]*TDecSbac)}
}

func (this *TDecSlice) Init(pcEntropyDecoder *TDecEntropy, pcCuDecoder *TDecCu) {
  this.m_pcEntropyDecoder  = pcEntropyDecoder;
  this.m_pcCuDecoder       = pcCuDecoder;
}

func (this *TDecSlice) Create( pcSlice *TLibCommon.TComSlice, iWidth, iHeight int, uiMaxWidth, uiMaxHeight, uiMaxDepth uint){
	//do nothing
}
func (this *TDecSlice) Destroy() {
	//do nothing
}

func (this *TDecSlice) DecompressSlice   ( pcBitstream *TLibCommon.TComInputBitstream, ppcSubstreams []*TLibCommon.TComInputBitstream,   
										   rpcPic *TLibCommon.TComPic, pcSbacDecoder *TDecSbac, pcSbacDecoders []TDecSbac){
  //var pcCU *TLibCommon.TComDataCU;
  uiIsLast := uint(0);
  var iStartCUEncOrder uint 
  if rpcPic.GetSlice(rpcPic.GetCurrSliceIdx()).GetSliceCurStartCUAddr()/rpcPic.GetNumPartInCU() > rpcPic.GetSlice(rpcPic.GetCurrSliceIdx()).GetDependentSliceCurStartCUAddr()/rpcPic.GetNumPartInCU() {
  	iStartCUEncOrder = rpcPic.GetSlice(rpcPic.GetCurrSliceIdx()).GetSliceCurStartCUAddr()/rpcPic.GetNumPartInCU();
  }else{ 
  	iStartCUEncOrder = rpcPic.GetSlice(rpcPic.GetCurrSliceIdx()).GetDependentSliceCurStartCUAddr()/rpcPic.GetNumPartInCU();
  }
  iStartCUAddr := int(rpcPic.GetPicSym().GetCUOrderMap(int(iStartCUEncOrder)));

  // decoder don't need prediction & residual frame buffer
  rpcPic.SetPicYuvPred( nil );
  rpcPic.SetPicYuvResi( nil );
  
//#if ENC_DEC_TRACE
//  g_bJustDoIt = g_bEncDecTraceEnable;
//#endif
//  DTRACE_CABAC_VL( g_nSymbolCounter++ );
//  DTRACE_CABAC_T( "\tPOC: " );
//  DTRACE_CABAC_V( rpcPic.GetPOC() );
//  DTRACE_CABAC_T( "\n" );
//#if ENC_DEC_TRACE
//  g_bJustDoIt = g_bEncDecTraceDisable;
//#endif

  uiTilesAcross := rpcPic.GetPicSym().GetNumColumnsMinus1()+1;
  pcSlice 		:= rpcPic.GetSlice(rpcPic.GetCurrSliceIdx());
  iNumSubstreams:= pcSlice.GetPPS().GetNumSubstreams();

  // delete decoders if already allocated in previous slice
  /*if (m_pcBufferSbacDecoders)
  {
    delete [] m_pcBufferSbacDecoders;
  }
  if (m_pcBufferBinCABACs) 
  {
    delete [] m_pcBufferBinCABACs;
  }*/
  // allocate new decoders based on tile numbaer
  this.m_pcBufferSbacDecoders = make([]*TDecSbac,	uiTilesAcross);  
  this.m_pcBufferBinCABACs    = make([]*TDecBinCabac,uiTilesAcross);
  for ui := 0; ui < uiTilesAcross; ui++ {
  	this.m_pcBufferBinCABACs   [ui] = NewTDecBinCabac();
  	this.m_pcBufferSbacDecoders[ui] = NewTDecSbac();
    this.m_pcBufferSbacDecoders[ui].Init(this.m_pcBufferBinCABACs[ui]);
  }
  //save init. state
  for ui := 0; ui < uiTilesAcross; ui++ {
    this.m_pcBufferSbacDecoders[ui].Load(pcSbacDecoder);
  }

  // free memory if already allocated in previous call
  /*if (this.m_pcBufferLowLatSbacDecoders)
  {
    delete [] this.m_pcBufferLowLatSbacDecoders;
  }
  if (this.m_pcBufferLowLatBinCABACs)
  {
    delete [] this.m_pcBufferLowLatBinCABACs;
  }*/
  this.m_pcBufferLowLatSbacDecoders = make([]*TDecSbac,    uiTilesAcross);  
  this.m_pcBufferLowLatBinCABACs    = make([]*TDecBinCabac,uiTilesAcross);
  for ui := 0; ui < uiTilesAcross; ui++ {
  	this.m_pcBufferLowLatBinCABACs	 [ui] = NewTDecBinCabac();
  	this.m_pcBufferLowLatSbacDecoders[ui] = NewTDecSbac();
    this.m_pcBufferLowLatSbacDecoders[ui].Init(this.m_pcBufferLowLatBinCABACs[ui]);
  }
  //save init. state
  for ui := 0; ui < uiTilesAcross; ui++ {
    this.m_pcBufferLowLatSbacDecoders[ui].Load(pcSbacDecoder);
  }

  uiWidthInLCUs  := rpcPic.GetPicSym().GetFrameWidthInCU();
  //UInt uiHeightInLCUs = rpcPic.GetPicSym().GetFrameHeightInCU();
  uiCol :=uint(0);
  uiLin :=uint(0);
  uiSubStrm :=uint(0);

  var uiTileCol, uiTileStartLCU, uiTileLCUX uint;
  iNumSubstreamsPerTile := 1; // if independent.
//#if DEPENDENT_SLICES
  bAllowDependence := false;
//#if REMOVE_ENTROPY_SLICES
  if rpcPic.GetSlice(rpcPic.GetCurrSliceIdx()).GetPPS().GetDependentSliceEnabledFlag() {
//#else
//  if( rpcPic.GetSlice(rpcPic.GetCurrSliceIdx()).GetPPS().GetDependentSliceEnabledFlag()&& (!rpcPic.GetSlice(rpcPic.GetCurrSliceIdx()).GetPPS().GetEntropySliceEnabledFlag()) )
//#endif
    bAllowDependence = true;
  }
  if bAllowDependence {
    if !rpcPic.GetSlice(rpcPic.GetCurrSliceIdx()).IsNextSlice() {
      uiTileCol = 0;
      if pcSlice.GetPPS().GetEntropyCodingSyncEnabledFlag() {
        this.m_pcBufferSbacDecoders[uiTileCol].LoadContexts( this.CTXMem[1]  );//2.LCU
      }
      pcSbacDecoder.LoadContexts(this.CTXMem[0] ); //end of depSlice-1
      pcSbacDecoders[uiSubStrm].LoadContexts(pcSbacDecoder);
    }else{
      if pcSlice.GetPPS().GetEntropyCodingSyncEnabledFlag() {
        this.CTXMem[1].LoadContexts(pcSbacDecoder);
      }
      this.CTXMem[0].LoadContexts(pcSbacDecoder);
    }
  }
//#endif

  for iCUAddr := iStartCUAddr; uiIsLast==0 && iCUAddr < int(rpcPic.GetNumCUsInFrame()); iCUAddr = int(rpcPic.GetPicSym().XCalculateNxtCUAddr(uint(iCUAddr))) {
    pcCU := rpcPic.GetCU( uint(iCUAddr) );
    pcCU.InitCU( rpcPic, uint(iCUAddr) );

//#ifdef ENC_DEC_TRACE
    pcSbacDecoder.XTraceLCUHeader(TLibCommon.TRACE_LCU);
    pcSbacDecoder.XReadAeTr (iCUAddr, "lcu_address", TLibCommon.TRACE_LCU);
	pcSbacDecoder.XReadAeTr (int(rpcPic.GetPicSym().GetTileIdxMap(iCUAddr)), "tile_id", TLibCommon.TRACE_LCU);
//#endif

    uiTileCol = rpcPic.GetPicSym().GetTileIdxMap(int(iCUAddr)) % uint(rpcPic.GetPicSym().GetNumColumnsMinus1()+1); // what column of tiles are we in?
    uiTileStartLCU = rpcPic.GetPicSym().GetTComTile(rpcPic.GetPicSym().GetTileIdxMap(int(iCUAddr))).GetFirstCUAddr();
    uiTileLCUX = uiTileStartLCU % uiWidthInLCUs;
    uiCol     = uint(iCUAddr) % uiWidthInLCUs;
    // The 'line' is now relative to the 1st line in the slice, not the 1st line in the picture.
    uiLin     = (uint(iCUAddr)/uiWidthInLCUs)-(uint(iStartCUAddr)/uiWidthInLCUs);
    // inherit from TR if necessary, select substream to use.
//#if DEPENDENT_SLICES
    if (pcSlice.GetPPS().GetNumSubstreams() > 1) || ( bAllowDependence  && (uiCol == uiTileLCUX)&&(pcSlice.GetPPS().GetEntropyCodingSyncEnabledFlag()) ){
//#else
//    if( pcSlice.GetPPS().GetNumSubstreams() > 1 )
//#endif
      // independent tiles => substreams are "per tile".  iNumSubstreams has already been multiplied.
      iNumSubstreamsPerTile = iNumSubstreams/rpcPic.GetPicSym().GetNumTiles();
      uiSubStrm = rpcPic.GetPicSym().GetTileIdxMap(iCUAddr)*uint(iNumSubstreamsPerTile) + uiLin%uint(iNumSubstreamsPerTile);
      this.m_pcEntropyDecoder.SetBitstream( ppcSubstreams[uiSubStrm] );
      // Synchronize cabac probabilities with upper-right LCU if it's available and we're at the start of a line.
//#if DEPENDENT_SLICES
      if ((pcSlice.GetPPS().GetNumSubstreams() > 1) || bAllowDependence ) && (uiCol == uiTileLCUX)&&(pcSlice.GetPPS().GetEntropyCodingSyncEnabledFlag()) {
//#else
//      if (pcSlice.GetPPS().GetNumSubstreams() > 1 && uiCol == uiTileLCUX)
//#endif
        // We'll sync if the TR is available.
        pcCUUp := pcCU.GetCUAbove();
        uiWidthInCU := rpcPic.GetFrameWidthInCU();
        var pcCUTR *TLibCommon.TComDataCU;
        if  pcCUUp!=nil && ((uint(iCUAddr)%uiWidthInCU+1) < uiWidthInCU)  {
          pcCUTR = rpcPic.GetCU( uint(iCUAddr) - uiWidthInCU + 1 );
        }
        uiMaxParts := uint(1<<(pcSlice.GetSPS().GetMaxCUDepth()<<1));

        if  (true && //bEnforceSliceRestriction
             ((pcCUTR==nil) || (pcCUTR.GetSlice()==nil) || 
             ((pcCUTR.GetSCUAddr()+uiMaxParts-1) < pcSlice.GetSliceCurStartCUAddr()) ||
             ((rpcPic.GetPicSym().GetTileIdxMap( int(pcCUTR.GetAddr()) ) != rpcPic.GetPicSym().GetTileIdxMap(iCUAddr)))))||
             (true && //bEnforceDependentSliceRestriction
             ((pcCUTR==nil) || (pcCUTR.GetSlice()==nil) || 
             ((pcCUTR.GetSCUAddr()+uiMaxParts-1) < pcSlice.GetDependentSliceCurStartCUAddr()) ||
             ((rpcPic.GetPicSym().GetTileIdxMap( int(pcCUTR.GetAddr()) ) != rpcPic.GetPicSym().GetTileIdxMap(iCUAddr))))) {
//#if DEPENDENT_SLICES
          if (iCUAddr!=0) && pcCUTR!=nil && ((pcCUTR.GetSCUAddr()+uiMaxParts-1) >= pcSlice.GetSliceCurStartCUAddr()) && bAllowDependence {
             pcSbacDecoders[uiSubStrm].LoadContexts( this.m_pcBufferSbacDecoders[uiTileCol] ); 
          }
//#endif
          // TR not available.
        }else{
          // TR is available, we use it.
          pcSbacDecoders[uiSubStrm].LoadContexts( this.m_pcBufferSbacDecoders[uiTileCol] );
        }
      }
      pcSbacDecoder.Load(&pcSbacDecoders[uiSubStrm]);  //this load is used to simplify the code (avoid to change all the call to pcSbacDecoders)
    }else if pcSlice.GetPPS().GetNumSubstreams() <= 1 {
      // Set variables to appropriate values to avoid later code change.
      iNumSubstreamsPerTile = 1;
    }

    if   (uint(iCUAddr) == rpcPic.GetPicSym().GetTComTile(rpcPic.GetPicSym().GetTileIdxMap(iCUAddr)).GetFirstCUAddr()) && // 1st in tile.
         (iCUAddr!=0) && (uint(iCUAddr)!=rpcPic.GetPicSym().GetPicSCUAddr(rpcPic.GetSlice(rpcPic.GetCurrSliceIdx()).GetSliceCurStartCUAddr())/rpcPic.GetNumPartInCU()) && 
//#if DEPENDENT_SLICES
         (uint(iCUAddr)!=rpcPic.GetPicSym().GetPicSCUAddr(rpcPic.GetSlice(rpcPic.GetCurrSliceIdx()).GetDependentSliceCurStartCUAddr())/rpcPic.GetNumPartInCU()) {
//#endif
          // !1st in frame && !1st in slice
      if pcSlice.GetPPS().GetNumSubstreams() > 1 {
        // We're crossing into another tile, tiles are independent.
        // When tiles are independent, we have "substreams per tile".  Each substream has already been terminated, and we no longer
        // have to perform it here.
        // For TILES_DECODER, there can be a header at the start of the 1st substream in a tile.  These are read when the substreams
        // are extracted, not here.
      }else {
        sliceType  := pcSlice.GetSliceType();
        if pcSlice.GetCabacInitFlag() {
          switch sliceType {
          case TLibCommon.P_SLICE:           // change initialization table to B_SLICE intialization
            sliceType = TLibCommon.B_SLICE; 
            //break;
          case TLibCommon.B_SLICE:           // change initialization table to P_SLICE intialization
            sliceType = TLibCommon.P_SLICE; 
            //break;
          //default     :           // should not occur
            //assert(0);
          }
        }
        this.m_pcEntropyDecoder.UpdateContextTables( sliceType, pcSlice.GetSliceQp() );
      }
    }

//#if ENC_DEC_TRACE
//    g_bJustDoIt = g_bEncDecTraceEnable;
//#endif
    if pcSlice.GetSPS().GetUseSAO() && (pcSlice.GetSaoEnabledFlag()||pcSlice.GetSaoEnabledFlagChroma())  {
      saoParam := rpcPic.GetPicSym().GetSaoParam();
      saoParam.SaoFlag[0] = pcSlice.GetSaoEnabledFlag();
      if iCUAddr == iStartCUAddr {
        saoParam.SaoFlag[1] = pcSlice.GetSaoEnabledFlagChroma();
      }
      numCuInWidth    := saoParam.NumCuInWidth;
      cuAddrInSlice   := iCUAddr - int(rpcPic.GetPicSym().GetCUOrderMap(int(pcSlice.GetSliceCurStartCUAddr()/rpcPic.GetNumPartInCU())));
      cuAddrUpInSlice := cuAddrInSlice - numCuInWidth;
      rx := iCUAddr % numCuInWidth;
      ry := iCUAddr / numCuInWidth;
      allowMergeLeft := true;
      allowMergeUp   := true;
      if rx!=0 {
        if rpcPic.GetPicSym().GetTileIdxMap(iCUAddr-1) != rpcPic.GetPicSym().GetTileIdxMap(iCUAddr) {
          allowMergeLeft = false;
        }
      }
      if ry!=0 {
        if rpcPic.GetPicSym().GetTileIdxMap(iCUAddr-numCuInWidth) != rpcPic.GetPicSym().GetTileIdxMap(iCUAddr) {
          allowMergeUp = false;
        }
      }
      pcSbacDecoder.ParseSaoOneLcuInterleaving(rx, ry, saoParam,pcCU, cuAddrInSlice, cuAddrUpInSlice, allowMergeLeft, allowMergeUp);
    }
    this.m_pcCuDecoder.DecodeCU     ( pcCU, &uiIsLast );
    this.m_pcCuDecoder.DecompressCU ( pcCU );
    
//#if ENC_DEC_TRACE
//    g_bJustDoIt = g_bEncDecTraceDisable;
//#endif
    pcSbacDecoders[uiSubStrm].Load(pcSbacDecoder);

    //Store probabilities of second LCU in line into buffer
//#if DEPENDENT_SLICES
    if (uiCol == uiTileLCUX+1)&& (bAllowDependence || (pcSlice.GetPPS().GetNumSubstreams() > 1)) && (pcSlice.GetPPS().GetEntropyCodingSyncEnabledFlag()) {
//#else
//    if (pcSlice.GetPPS().GetNumSubstreams() > 1 && (uiCol == uiTileLCUX+1))
//#endif
      this.m_pcBufferSbacDecoders[uiTileCol].LoadContexts( &pcSbacDecoders[uiSubStrm] );
    }
//#if DEPENDENT_SLICES
    if uiIsLast!=0 && bAllowDependence {
      if pcSlice.GetPPS().GetEntropyCodingSyncEnabledFlag() {
         this.CTXMem[1].LoadContexts( this.m_pcBufferSbacDecoders[uiTileCol] );//ctx 2.LCU
      }
      this.CTXMem[0].LoadContexts( pcSbacDecoder );//ctx end of dep.slice
      return;
    }
//#endif
  }
  
  return
}

//#if DEPENDENT_SLICES
func (this *TDecSlice)  InitCtxMem(  i uint){
  for j := 0; j < len(this.CTXMem); j++ {
    delete (this.CTXMem, j);
  }
  
  this.CTXMem = make(map[int]*TDecSbac, i);
}
func (this *TDecSlice)  SetCtxMem( sb *TDecSbac, b int )   { 
	this.CTXMem[b] = sb; 
}
//#endif
//};
/*
type ParameterSetManagerDecoder struct {
    TLibCommon.ParameterSetManager
    //private:
    //  ParameterSetMap<TComVPS> m_vpsBuffer;
    //  ParameterSetMap<TComSPS> m_spsBuffer; 
    //  ParameterSetMap<TComPPS> m_ppsBuffer;
}


func NewParameterSetManagerDecoder() *ParameterSetManagerDecoder{
	return ParameterSetManagerDecoder{TLibCommon.ParameterSetManager{make(map[int]*TLibCommon.TComVPS), 
																	 make(map[int]*TLibCommon.TComSPS), 
																	 make(map[int]*TLibCommon.TComPPS)}}
}

func (this *ParameterSetManagerDecoder)  SetPrefetchedVPS(vps *TLibCommon.TComVPS)  { 
	this.SetVPS(vps); 
}
func (this *ParameterSetManagerDecoder)  GetPrefetchedVPS  (vpsId int) *TLibCommon.TComVPS {
	return this.GetVPS(vpsId)
}
func (this *ParameterSetManagerDecoder)  SetPrefetchedSPS(sps *TLibCommon.TComSPS)  { 
	this.SetSPS(sps); 
};
func (this *ParameterSetManagerDecoder)  GetPrefetchedSPS  (spsId int) *TLibCommon.TComSPS{
	return this.GetSPS(spsId)
}
func (this *ParameterSetManagerDecoder)  SetPrefetchedPPS(pps *TLibCommon.TComPPS)  { 
	this.SetPPS(pps); 
}
func (this *ParameterSetManagerDecoder)  GetPrefetchedPPS  (ppsId int) *TLibCommon.TComPPS{
	return this.GetPPS(ppsId)
}
func (this *ParameterSetManagerDecoder)  ApplyPrefetchedPS() {
}*/
